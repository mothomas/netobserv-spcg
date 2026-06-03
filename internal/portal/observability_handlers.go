package portal

import (
	"encoding/json"
	"net/http"

	"github.com/netobserv/spcg/internal/pcap"
)

func (s *Server) registerObservabilityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/capture/observability", s.handleCaptureObservability)
}

func (s *Server) handleCaptureObservability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		CaptureSessionID string `json:"capture_session_id"`
		SessionID        string `json:"session_id"`
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

	events := sess.Events()
	tracked := sess.TrackedPods()
	topo, graphCapped := boundedTopologyFromSession(sess)
	summary := pcap.BuildCaptureSummary(events, tracked)

	writeJSON(w, map[string]interface{}{
		"session_id":        captureID,
		"event_count":       len(events),
		"topology":          topo,
		"capture_summary":   summary,
		"tracked_pod_ids":   sess.TrackedPodIDList(),
		"s3_export":         sess.S3Export(),
		"graph_capped":      graphCapped,
		"events_sampled":    len(events) > pcap.MaxTopologyBuildEvents,
	})
}
