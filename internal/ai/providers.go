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

// Provider identifies an upstream LLM integration.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderOllama    Provider = "ollama"
	ProviderOpenAICompat Provider = "openai_compatible" // Azure OpenAI / Copilot Studio gateways
	ProviderCursor       Provider = "cursor"          // Cursor Cloud Agents API (no-repo)
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Provider       Provider
	Model          string
	APIEndpoint    string
	APIKey         string
	ProxyURL       string
	Messages       []ChatMessage
	CursorAgentID  string // follow-up runs on same Cloud Agent
}

type ChatResponse struct {
	Reply          string `json:"reply"`
	RawModelOut    string `json:"raw_model_out,omitempty"`
	CursorAgentID  string `json:"cursor_agent_id,omitempty"`
}

type ChatClient struct {
	HTTP *http.Client
}

func NewChatClient(proxyURL string) *ChatClient {
	return &ChatClient{HTTP: newProxyHTTP(proxyURL, 180)}
}

func (c *ChatClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}
	switch req.Provider {
	case ProviderOpenAI, ProviderOpenAICompat:
		return c.chatOpenAI(ctx, req)
	case ProviderAnthropic:
		return c.chatAnthropic(ctx, req)
	case ProviderGemini:
		return c.chatGemini(ctx, req)
	case ProviderOllama:
		return c.chatOllama(ctx, req)
	case ProviderCursor:
		reply, agentID, err := ChatCursor(ctx, c.HTTP, req, req.CursorAgentID)
		if err != nil {
			return nil, err
		}
		if agentID != "" {
			reply.CursorAgentID = agentID
		}
		return reply, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}
}

func (c *ChatClient) chatOpenAI(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	endpoint := req.APIEndpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": req.Messages,
	})
	return c.postJSON(ctx, endpoint, req.APIKey, body)
}

func (c *ChatClient) chatAnthropic(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	endpoint := req.APIEndpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}
	model := req.Model
	if model == "" {
		model = "claude-3-5-haiku-20241022"
	}
	var system string
	var msgs []map[string]string
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		msgs = append(msgs, map[string]string{"role": role, "content": m.Content})
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 4096,
		"system":     system,
		"messages":   msgs,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return c.doRequest(httpReq)
}

func (c *ChatClient) chatGemini(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	endpoint := req.APIEndpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	}
	var system string
	contents := make([]map[string]interface{}, 0)
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	bodyMap := map[string]interface{}{"contents": contents}
	if system != "" {
		bodyMap["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": strings.TrimSpace(system)}},
		}
	}
	body, _ := json.Marshal(bodyMap)
	url := endpoint
	if !strings.Contains(url, "key=") && req.APIKey != "" {
		if strings.Contains(url, "?") {
			url += "&key=" + req.APIKey
		} else {
			url += "?key=" + req.APIKey
		}
	}
	return c.postJSON(ctx, url, "", body)
}

func (c *ChatClient) chatOllama(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	endpoint := req.APIEndpoint
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434/api/chat"
	}
	model := req.Model
	if model == "" {
		model = "llama3.2"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   false,
	})
	return c.postJSON(ctx, endpoint, req.APIKey, body)
}

func (c *ChatClient) postJSON(ctx context.Context, endpoint, token string, body []byte) (*ChatResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	return c.doRequest(httpReq)
}

func (c *ChatClient) doRequest(httpReq *http.Request) (*ChatResponse, error) {
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(raw))
	}
	reply := extractSummary(string(raw))
	return &ChatResponse{Reply: reply, RawModelOut: string(raw)}, nil
}

func newProxyHTTP(proxyURL string, timeoutSec int) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		if u, err := parseProxy(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	sec := timeoutSec
	if sec <= 0 {
		sec = 180
	}
	return &http.Client{Transport: tr, Timeout: time.Duration(sec) * time.Second}
}

func parseProxy(proxyURL string) (*url.URL, error) {
	return url.Parse(proxyURL)
}
