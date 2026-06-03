package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/netobserv/spcg/internal/ai"
	"github.com/netobserv/spcg/internal/auth"
	"github.com/netobserv/spcg/internal/pcap"
)

var (
	chatHistMu sync.Mutex
	chatHist   = make(map[string][]ai.ChatMessage)
)

func (s *Server) registerAIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ai/verify", s.handleAIVerify)
	mux.HandleFunc("/api/v1/ai/context", s.handleAIContext)
	mux.HandleFunc("/api/v1/ai/chat", s.handleAIChat)
	mux.HandleFunc("/api/v1/ai/triage", s.handleAITriage)
}

func (s *Server) requireCaptureAccess(w http.ResponseWriter, r *http.Request, captureID string) bool {
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return false
	}
	if captureID == "" || !assertCaptureOwner(captureID, authSID) {
		http.Error(w, "capture session not found or access denied", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) handleAIVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		CaptureSessionID string `json:"capture_session_id"`
		Provider         string `json:"provider"`
		Model            string `json:"model"`
		ProxyURL         string `json:"proxy_url"`
		APIEndpoint      string `json:"api_endpoint"`
		APIKey           string `json:"api_key"`
		TestLLM          bool   `json:"test_llm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	out := map[string]interface{}{
		"auth_ok": false, "capture_ok": false, "llm_ok": false, "ready": false,
	}

	if _, err := s.userClient(r); err != nil {
		out["auth_error"] = err.Error()
		writeJSON(w, out)
		return
	}
	out["auth_ok"] = true

	if body.CaptureSessionID == "" {
		out["capture_error"] = "capture_session_id is required"
		writeJSON(w, out)
		return
	}
	if !s.requireCaptureAccess(w, r, body.CaptureSessionID) {
		return
	}
	pcapSess, ok := pcap.Get(body.CaptureSessionID)
	if !ok {
		out["capture_error"] = "capture session not found in portal memory (re-run capture or refresh)"
		writeJSON(w, out)
		return
	}
	events := pcapSess.Events()
	out["capture_ok"] = true
	out["capture_events"] = len(events)
	out["capture_bytes"] = pcapSess.TotalBytes()

	if body.TestLLM {
		if body.APIKey == "" {
			out["llm_error"] = "api_key is required to test LLM connectivity"
			writeJSON(w, out)
			return
		}
		timeout := 45 * time.Second
		if body.Provider == string(ai.ProviderCursor) {
			timeout = 120 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		client := ai.NewChatClient(body.ProxyURL)
		var cursorAgentID string
		aiSessionsMu.Lock()
		if c, ok := aiSessions[body.CaptureSessionID]; ok {
			cursorAgentID = c.cursorAgentID
		}
		aiSessionsMu.Unlock()
		resp, err := client.Chat(ctx, ai.ChatRequest{
			Provider:      ai.Provider(body.Provider),
			Model:         body.Model,
			APIEndpoint:   body.APIEndpoint,
			APIKey:        body.APIKey,
			ProxyURL:      body.ProxyURL,
			CursorAgentID: cursorAgentID,
			Messages: []ai.ChatMessage{
				{Role: "system", Content: "Reply with exactly: OK"},
				{Role: "user", Content: "ping"},
			},
		})
		if err != nil {
			out["llm_error"] = err.Error()
			writeJSON(w, out)
			return
		}
		out["llm_ok"] = true
		out["llm_preview"] = truncate(resp.Reply, 200)
	}

	out["ready"] = out["auth_ok"].(bool) && out["capture_ok"].(bool)
	if body.TestLLM {
		out["ready"] = out["ready"].(bool) && out["llm_ok"].(bool)
	}
	writeJSON(w, out)
}

func (s *Server) handleAIContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		SessionID        string `json:"session_id"`
		CaptureSessionID string `json:"capture_session_id"`
		MaxLines         int    `json:"max_lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	captureID := body.CaptureSessionID
	if captureID == "" {
		captureID = body.SessionID
	}
	if !s.requireCaptureAccess(w, r, captureID) {
		return
	}
	sess, ok := pcap.Get(captureID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	scrub := ai.ScrubberForSession(captureID)
	jsonl, err := sess.ExportJSONL(scrub, body.MaxLines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	events := sess.Events()
	tracked := sess.TrackedPods()
	topo, _ := pcap.BuildBoundedTopology(events, tracked)
	graph, _ := pcap.MergeTopologyIntoGraph(pcap.SampleEventsForGraph(events, pcap.MaxTopologyBuildEvents), tracked)
	if authSID, err := s.authSessionID(r); err == nil {
		go s.syncTopologyToGraph(captureID, authSID, topo, tracked)
	}
	summary := pcap.BuildCaptureSummary(events, tracked)
	lineCount := 0
	if len(jsonl) > 0 {
		for _, b := range jsonl {
			if b == '\n' {
				lineCount++
			}
		}
	}
	graphCtx := buildScrubbedGraphContext(scrub, topo)
	writeJSON(w, map[string]interface{}{
		"session_id":       captureID,
		"event_count":      len(events),
		"jsonl_lines":      lineCount,
		"jsonl_preview":    truncate(string(jsonl), 12000),
		"flow_graph":       graph,
		"topology":         topo,
		"capture_summary":  summary,
		"tracked_pod_ids":  sess.TrackedPodIDList(),
		"scrub_legend":     scrub.SnapshotMap(),
		"graph_context":    graphCtx,
		"s3_export":        sess.S3Export(),
	})
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		SessionID        string `json:"session_id"`
		CaptureSessionID string `json:"capture_session_id"`
		Message          string `json:"message"`
		Provider    string `json:"provider"`
		Model       string `json:"model"`
		ProxyURL    string `json:"proxy_url"`
		APIEndpoint string `json:"api_endpoint"`
		APIKey      string `json:"api_key"`
		Flush       bool   `json:"flush_session"`
		ResetChat   bool   `json:"reset_chat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	captureID := body.CaptureSessionID
	if captureID == "" {
		captureID = body.SessionID
	}
	if captureID != "" && !s.requireCaptureAccess(w, r, captureID) {
		return
	}

	if body.Flush {
		ai.DropScrubber(captureID)
		chatHistMu.Lock()
		delete(chatHist, captureID)
		chatHistMu.Unlock()
		aiSessionsMu.Lock()
		if c, ok := aiSessions[captureID]; ok {
			auth.Wipe(c.bearer)
			delete(aiSessions, captureID)
		}
		aiSessionsMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	scrub := ai.ScrubberForSession(captureID)
	pcapSess, ok := pcap.Get(captureID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if body.ResetChat {
		chatHistMu.Lock()
		delete(chatHist, captureID)
		chatHistMu.Unlock()
	}

	jsonl, _ := pcapSess.ExportJSONL(scrub, 400)
	events := pcapSess.Events()
	tracked := pcapSess.TrackedPods()
	topo, _ := pcap.BuildBoundedTopology(events, tracked)
	graphCtx := buildScrubbedGraphContext(scrub, topo)
	msgs := ai.BuildTriageMessages(string(jsonl), graphCtx, "")

	chatHistMu.Lock()
	hist := chatHist[captureID]
	if len(hist) == 0 {
		hist = msgs
	} else if body.Message != "" {
		hist = append(hist, ai.ChatMessage{Role: "user", Content: scrub.Scrub(body.Message)})
	}
	chatHist[captureID] = hist
	chatHistMu.Unlock()

	if body.Message == "" && len(hist) <= len(msgs) {
		writeJSON(w, map[string]interface{}{
			"reply":           "",
			"capture_summary": pcap.BuildCaptureSummary(events, tracked),
			"jsonl_lines":     countLines(jsonl),
			"scrub_legend":    scrub.SnapshotMap(),
			"graph_context":   graphCtx,
		})
		return
	}

	apiKey := []byte(body.APIKey)
	aiSessionsMu.Lock()
	creds := aiSessions[captureID]
	if creds == nil {
		creds = &aiSessionCreds{}
		aiSessions[captureID] = creds
	}
	creds.proxyURL = body.ProxyURL
	creds.targetType = ai.TargetType(body.Provider)
	creds.apiEndpoint = body.APIEndpoint
	if body.APIKey != "" {
		creds.bearer = apiKey
	}
	cursorAgentID := creds.cursorAgentID
	aiSessionsMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	client := ai.NewChatClient(body.ProxyURL)
	resp, err := client.Chat(ctx, ai.ChatRequest{
		Provider:      ai.Provider(body.Provider),
		Model:         body.Model,
		APIEndpoint:   body.APIEndpoint,
		APIKey:        body.APIKey,
		ProxyURL:      body.ProxyURL,
		CursorAgentID: cursorAgentID,
		Messages:      hist,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	reply := scrub.Restore(resp.Reply)
	if resp.CursorAgentID != "" {
		aiSessionsMu.Lock()
		if c := aiSessions[captureID]; c != nil {
			c.cursorAgentID = resp.CursorAgentID
		}
		aiSessionsMu.Unlock()
	}
	chatHistMu.Lock()
	hist = append(chatHist[captureID], ai.ChatMessage{Role: "assistant", Content: reply})
	chatHist[captureID] = hist
	chatHistMu.Unlock()

	writeJSON(w, map[string]interface{}{
		"reply":           reply,
		"capture_summary": pcap.BuildCaptureSummary(events, tracked),
		"scrub_legend":    scrub.SnapshotMap(),
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(truncated)"
}

func countLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	n := 1
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
