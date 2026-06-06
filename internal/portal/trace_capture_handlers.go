package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/pcap"
	"github.com/netobserv/spcg/internal/trace"
)

func (s *Server) registerTraceCaptureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/trace/capture/start", s.handleTraceCaptureStart)
	mux.HandleFunc("/api/v1/trace/capture/stop", s.handleTraceCaptureStop)
	mux.HandleFunc("/api/v1/trace/status", s.handleTraceStatus)
}

func (s *Server) handleTraceCaptureStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	var body struct {
		TraceID string `json:"trace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	traceID := strings.TrimSpace(body.TraceID)
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
	if existing := traceCaptureSessionID(traceID); existing != "" {
		if _, ok := pcap.Get(existing); ok {
			writeJSON(w, map[string]interface{}{
				"trace_id":            traceID,
				"capture_session_id":  existing,
				"capture_active":      captureStreamActive(existing),
				"source_pods":         len(sess.Response.SourcePods),
				"already_running":     true,
			})
			return
		}
		stopTraceCapture(traceID)
	}

	selections := traceCaptureSelections(sess.Response)
	if len(selections) == 0 {
		http.Error(w, "trace has no source pods to capture", http.StatusBadRequest)
		return
	}
	cs, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resolved, err := spcgk8s.ResolveCaptureSelections(r.Context(), cs, selections)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessNS := ""
	if len(sess.Response.Graph.Namespaces) > 0 {
		sessNS = sess.Response.Graph.Namespaces[0]
	}
	prep, err := s.prepareCaptureIngest(r.Context(), authSID, resolved, sessNS, pcap.S3CaptureConfig{Enabled: false})
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	linkTraceCapture(traceID, prep.Session.ID, cancel)
	go s.runCaptureIngest(ctx, prep)

	writeJSON(w, map[string]interface{}{
		"trace_id":           traceID,
		"capture_session_id": prep.Session.ID,
		"capture_active":     true,
		"resolved_pods":      len(resolved.Pods),
		"sensor_filters":     len(resolved.SensorTargets),
	})
}

func (s *Server) handleTraceCaptureStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		TraceID string `json:"trace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	traceID := strings.TrimSpace(body.TraceID)
	if traceID == "" {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	if !s.requireTraceAccess(w, r, traceID) {
		return
	}
	captureID := stopTraceCapture(traceID)
	if captureID != "" {
		releaseCaptureStream(captureID)
	}
	writeJSON(w, map[string]interface{}{
		"trace_id":           traceID,
		"capture_session_id": captureID,
		"status":             "stopped",
	})
}

func (s *Server) handleTraceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimSpace(r.URL.Query().Get("trace_id"))
	if r.Method == http.MethodPost {
		var body struct {
			TraceID string `json:"trace_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.TraceID != "" {
			traceID = body.TraceID
		}
	}
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
	captureID := traceCaptureSessionID(traceID)
	captureActive := captureID != "" && captureStreamActive(captureID)
	eventCount := 0
	if captureID != "" {
		if cap, ok := pcap.Get(captureID); ok {
			eventCount = len(cap.Events())
		}
	}
	writeJSON(w, map[string]interface{}{
		"trace_id":           traceID,
		"capture_session_id": captureID,
		"capture_active":     captureActive,
		"capture_events":     eventCount,
		"source":             sess.Response.Source,
		"destination":        sess.Response.Destination,
		"source_pods":        len(sess.Response.SourcePods),
	})
}

func captureStreamActive(captureID string) bool {
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	_, ok := activeCaptureStreams[captureID]
	return ok
}

func traceCaptureSelections(resp trace.DiscoverResponse) []spcgk8s.CaptureSelection {
	src := resp.Source
	if src.Mode == trace.EndpointNamespace {
		if src.Type == "owner" && src.OwnerKind != "" && src.OwnerName != "" {
			return []spcgk8s.CaptureSelection{{
				Namespace: src.Namespace, Type: "owner",
				OwnerKind: src.OwnerKind, OwnerName: src.OwnerName,
				LabelSelector: src.LabelSelector,
			}}
		}
		if src.PodName != "" {
			return []spcgk8s.CaptureSelection{{
				Namespace: src.Namespace, Type: "pod",
				PodName: src.PodName, PodUID: src.PodUID,
			}}
		}
	}
	out := make([]spcgk8s.CaptureSelection, 0, len(resp.SourcePods))
	seen := map[string]struct{}{}
	for _, p := range resp.SourcePods {
		key := p.Namespace + "/" + p.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, spcgk8s.CaptureSelection{
			Namespace: p.Namespace, Type: "pod", PodName: p.Name, PodUID: p.UID,
		})
	}
	return out
}
