package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	"github.com/netobserv/spcg/internal/pcap"
)

func (s *Server) registerGraphRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/graph/topology", s.handleGraphTopology)
}

func (s *Server) handleGraphTopology(w http.ResponseWriter, r *http.Request) {
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

	topo, capped := boundedTopologyFromSession(sess)
	tracked := sess.TrackedPods()
	g := graphdb.SigmaGraphFromTopology(captureID, topo, trackedNodeBoolMap(tracked))

	authSID, _ := s.authSessionID(r)
	if s.Graph != nil && s.Graph.Enabled() && !capped && len(topo.Nodes) <= 80 && authSID != "" {
		go s.syncTopologyToGraph(captureID, authSID, topo, tracked)
	}

	writeJSON(w, g)
}

func trackedNodeBoolMap(tracked []pcap.TrackedPod) map[string]bool {
	ids := map[string]bool{}
	for _, t := range tracked {
		if t.Namespace != "" && t.Name != "" {
			ids[t.Namespace+"/"+t.Name] = true
		}
	}
	return ids
}

func (s *Server) syncTopologyToGraph(captureID, authSessionID string, topo pcap.FlowTopology, tracked []pcap.TrackedPod) {
	if s.Graph == nil || !s.Graph.Enabled() {
		return
	}
	ids := trackedNodeBoolMap(tracked)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.Graph.ReplaceTopology(ctx, captureID, authSessionID, topo, ids)
}

// GraphStore is re-exported for main wiring.
type GraphStore = graphdb.Store
