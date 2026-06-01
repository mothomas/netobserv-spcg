package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const triageSystemPrompt = `Act as an elite Network Performance & Core Security Triage Engineer.
Analyze this privacy-scrubbed network packet capture trace text array.
Identify the exact structural failure point (e.g., asymmetrical TCP window drop, MTU mismatch truncation, unaligned TLS cipher handshakes, or asynchronous DNS timeouts).
Provide a plain-language executive summary detailing the failure root cause, followed by the exact structural remediation steps.`

type TargetType string

const (
	TargetOllama  TargetType = "ollama"
	TargetBedrock TargetType = "bedrock"
	TargetGateway TargetType = "gateway"
)

type TriageRequest struct {
	TraceText   string     `json:"trace_text"`
	ProxyURL    string     `json:"proxy_url"`
	TargetType  TargetType `json:"target_type"`
	APIEndpoint string     `json:"api_endpoint"`
	BearerToken string     `json:"bearer_token"`
}

type TriageResponse struct {
	Summary     string `json:"summary"`
	RawModelOut string `json:"raw_model_out,omitempty"`
}

type TriageClient struct {
	HTTP *http.Client
}

func NewTriageClient(proxyURL string) *TriageClient {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &TriageClient{HTTP: &http.Client{Transport: tr, Timeout: 120 * time.Second}}
}

func (c *TriageClient) Run(ctx context.Context, req TriageRequest) (*TriageResponse, error) {
	sanitized := Sanitize(req.TraceText)
	provider, model, endpoint := legacyTargetMapping(req.TargetType, req.APIEndpoint)
	chat := NewChatClient(req.ProxyURL)
	resp, err := chat.Chat(ctx, ChatRequest{
		Provider: provider, Model: model, APIEndpoint: endpoint, APIKey: req.BearerToken,
		Messages: BuildTriageMessages(sanitized, ""),
	})
	if err != nil {
		return nil, err
	}
	return &TriageResponse{Summary: resp.Reply, RawModelOut: resp.RawModelOut}, nil
}

func legacyTargetMapping(t TargetType, endpoint string) (Provider, string, string) {
	switch t {
	case TargetOllama:
		return ProviderOllama, "llama3.2", endpoint
	case TargetBedrock, TargetGateway:
		return ProviderOpenAICompat, "", endpoint
	default:
		return ProviderOpenAI, "", endpoint
	}
}

func (c *TriageClient) callOllama(ctx context.Context, endpoint, prompt, token string) (*TriageResponse, error) {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434/api/generate"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":  "llama3",
		"prompt": prompt,
		"stream": false,
	})
	return c.postJSON(ctx, endpoint, token, body)
}

func (c *TriageClient) callGeneric(ctx context.Context, endpoint, prompt, token string) (*TriageResponse, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("api_endpoint is required for gateway/bedrock targets")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": triageSystemPrompt},
			{"role": "user", "content": prompt},
		},
	})
	return c.postJSON(ctx, endpoint, token, body)
}

func (c *TriageClient) postJSON(ctx context.Context, endpoint, token string, body []byte) (*TriageResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed building AI triage request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed executing AI triage call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("failed reading AI triage response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI endpoint returned status %d", resp.StatusCode)
	}

	summary := extractSummary(string(raw))
	return &TriageResponse{Summary: summary, RawModelOut: string(raw)}, nil
}

func extractSummary(raw string) string {
	var anthropic struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal([]byte(raw), &anthropic) == nil && len(anthropic.Content) > 0 {
		return strings.TrimSpace(anthropic.Content[0].Text)
	}
	var gemini struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal([]byte(raw), &gemini) == nil && len(gemini.Candidates) > 0 && len(gemini.Candidates[0].Content.Parts) > 0 {
		return strings.TrimSpace(gemini.Candidates[0].Content.Parts[0].Text)
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) == nil {
		if r, ok := m["response"].(string); ok {
			return strings.TrimSpace(r)
		}
		if choices, ok := m["choices"].([]interface{}); ok && len(choices) > 0 {
			if c0, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := c0["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok {
						return strings.TrimSpace(content)
					}
				}
			}
		}
	}
	return strings.TrimSpace(raw)
}
