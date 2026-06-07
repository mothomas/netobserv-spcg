package portal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/netobserv/spcg/internal/trace/probe"
	"k8s.io/client-go/kubernetes"
)

func (s *Server) registerTraceProbeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/trace/probe/interfaces", s.handleTraceProbeInterfaces)
	mux.HandleFunc("/api/v1/trace/probe/fire", s.handleTraceProbeFire)
	mux.HandleFunc("/api/v1/trace/probe/events", s.handleTraceProbeEvents)
}

func (s *Server) handleTraceProbeInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimSpace(r.URL.Query().Get("trace_id"))
	if traceID == "" {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	if !s.requireTraceAccess(w, r, traceID) {
		return
	}
	sess, ok := getTraceSession(traceID)
	if !ok {
		http.Error(w, "trace session not found", http.StatusNotFound)
		return
	}
	csWrap, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	ifaces, err := probe.ListAttachInterfaces(r.Context(), csWrap.Interface, sess.Response.TargetPod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{
		"trace_id":   traceID,
		"pod":        sess.Response.TargetPod,
		"interfaces": ifaces,
	})
}

func (s *Server) handleTraceProbeFire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req probe.FireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.TraceID = strings.TrimSpace(req.TraceID)
	if req.TraceID == "" {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	if !s.requireTraceAccess(w, r, req.TraceID) {
		return
	}
	sess, ok := getTraceSession(req.TraceID)
	if !ok {
		http.Error(w, "trace session not found", http.StatusNotFound)
		return
	}
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	csWrap, _ := s.userClient(r)
	cfg, _ := s.userRESTConfig(r)
	var cs kubernetes.Interface
	if csWrap != nil {
		cs = csWrap.Interface
	}

	captureID := traceCaptureSessionID(req.TraceID)
	captureAutoStarted := false
	if !req.Simulate {
		if res, capErr := s.ensureTraceCapture(r.Context(), r, authSID, req.TraceID, sess); capErr == nil && res != nil {
			captureID = res.CaptureSessionID
			captureAutoStarted = !res.AlreadyRunning
		}
	}

	resp, err := probe.Fire(r.Context(), cs, cfg, sess.Response, req, captureID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if captureAutoStarted {
		resp.CaptureAutoStarted = true
	}
	writeJSON(w, resp)
}

func (s *Server) handleTraceProbeEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimSpace(r.URL.Query().Get("trace_id"))
	if traceID == "" {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	if !s.requireTraceAccess(w, r, traceID) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub, active := probe.SubscribeEvents(traceID)
	if !active {
		states := probe.EdgeStates(traceID)
		payload, _ := json.Marshal(map[string]interface{}{
			"type":        "snapshot",
			"trace_id":    traceID,
			"edge_states": states,
		})
		fmt.Fprintf(w, "event: probe\ndata: %s\n\n", payload)
		flusher.Flush()
		<-r.Context().Done()
		return
	}
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: probe\ndata: %s\n\n", payload)
			flusher.Flush()
			if ev.Type == "probe_finished" || ev.Type == "error" {
				time.Sleep(200 * time.Millisecond)
				return
			}
		}
	}
}
