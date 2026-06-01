package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultCursorAPIBase = "https://api.cursor.com"

// ChatCursor runs a no-repo Cloud Agent for packet-triage prompts.
// agentID is an optional existing agent for follow-up runs; returns updated agent id.
func ChatCursor(ctx context.Context, client *http.Client, req ChatRequest, agentID string) (*ChatResponse, string, error) {
	if client == nil {
		client = &http.Client{Timeout: 180 * time.Second}
	}
	base := strings.TrimSpace(req.APIEndpoint)
	if base == "" {
		base = defaultCursorAPIBase
	}
	if idx := strings.Index(base, "/v1/"); idx > 0 {
		base = base[:idx]
	}
	base = strings.TrimRight(base, "/")

	prompt := messagesToPrompt(req.Messages)
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "composer-2.5"
	}

	var runID string
	var newAgentID string
	if agentID != "" {
		newAgentID = agentID
		var err error
		runID, err = cursorCreateRun(ctx, client, base, req.APIKey, agentID, prompt)
		if err != nil {
			return nil, "", err
		}
		text, err := cursorPollRun(ctx, client, base, req.APIKey, agentID, runID)
		if err != nil {
			return nil, newAgentID, err
		}
		return &ChatResponse{Reply: text}, newAgentID, nil
	}

	agentID, runID, err := cursorCreateAgent(ctx, client, base, req.APIKey, prompt, model)
	if err != nil {
		return nil, "", err
	}
	newAgentID = agentID
	text, err := cursorPollRun(ctx, client, base, req.APIKey, agentID, runID)
	if err != nil {
		return nil, newAgentID, err
	}
	return &ChatResponse{Reply: text}, newAgentID, nil
}

func messagesToPrompt(msgs []ChatMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		role := strings.ToUpper(m.Role)
		if role == "" {
			role = "USER"
		}
		b.WriteString(role)
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(m.Content))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

func cursorCreateAgent(ctx context.Context, client *http.Client, base, apiKey, prompt, model string) (agentID, runID string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"prompt": map[string]string{"text": prompt},
		"model":  map[string]string{"id": model},
	})
	return cursorPostAgent(ctx, client, base+"/v1/agents", apiKey, body)
}

func cursorCreateRun(ctx context.Context, client *http.Client, base, apiKey, agentID, prompt string) (runID string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"prompt": map[string]string{"text": prompt},
	})
	_, runID, err = cursorPostAgent(ctx, client, base+"/v1/agents/"+agentID+"/runs", apiKey, body)
	return runID, err
}

func cursorPostAgent(ctx context.Context, client *http.Client, url, apiKey string, body []byte) (agentID, runID string, err error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setCursorAuth(httpReq, apiKey)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Cursor API %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Agent struct {
			ID          string `json:"id"`
			LatestRunID string `json:"latestRunId"`
		} `json:"agent"`
		Run struct {
			ID      string `json:"id"`
			AgentID string `json:"agentId"`
		} `json:"run"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", "", fmt.Errorf("Cursor API: invalid response")
	}
	agentID = out.Agent.ID
	runID = out.Run.ID
	if runID == "" {
		runID = out.Agent.LatestRunID
	}
	if agentID == "" && out.Run.AgentID != "" {
		agentID = out.Run.AgentID
	}
	if agentID == "" || runID == "" {
		return "", "", fmt.Errorf("Cursor API: missing agent or run id")
	}
	return agentID, runID, nil
}

func cursorPollRun(ctx context.Context, client *http.Client, base, apiKey, agentID, runID string) (string, error) {
	url := fmt.Sprintf("%s/v1/agents/%s/runs/%s", base, agentID, runID)
	deadline := time.Now().Add(170 * time.Second)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("Cursor agent run timed out")
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		setCursorAuth(httpReq, apiKey)
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("Cursor API %d: %s", resp.StatusCode, string(raw))
		}
		var run struct {
			Status string `json:"status"`
			Result string `json:"result"`
		}
		if json.Unmarshal(raw, &run) != nil {
			return "", fmt.Errorf("Cursor API: invalid run response")
		}
		switch strings.ToUpper(run.Status) {
		case "FINISHED":
			if strings.TrimSpace(run.Result) == "" {
				return "(Cursor agent finished with no text result)", nil
			}
			return run.Result, nil
		case "ERROR", "CANCELLED", "EXPIRED":
			return "", fmt.Errorf("Cursor agent run %s", run.Status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func setCursorAuth(req *http.Request, apiKey string) {
	key := CursorAPIKeyForBasic(apiKey)
	if key == "" {
		return
	}
	req.Header.Del("Authorization")
	req.SetBasicAuth(key, "")
}

// CursorAPIKeyForBasic strips Bearer prefix for Basic auth username.
func CursorAPIKeyForBasic(apiKey string) string {
	k := strings.TrimSpace(apiKey)
	k = strings.TrimPrefix(k, "Bearer ")
	k = strings.TrimPrefix(k, "bearer ")
	return k
}
