package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	"github.com/netobserv/spcg/internal/rbac"
	"github.com/netobserv/spcg/internal/trace"
	"github.com/netobserv/spcg/internal/trace/engine"
)

func (s *Server) registerTracePathRoute(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/trace/path", s.handleTracePath)
}

func (s *Server) handleTracePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	topo, sigma, err := s.runTracePath(r, authSID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeTracePathJSON(w, topo, sigma)
}

func (s *Server) runTracePath(r *http.Request, authSID string) (*engine.TopologyResult, *graphdb.SigmaGraph, error) {
	var req trace.DiscoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, nil, err
	}
	if len(req.Namespaces) == 0 {
		return nil, nil, errTraceNamespaces
	}
	if req.Source.Mode == "" {
		return nil, nil, errTraceSource
	}
	if req.Destination.Mode == "" {
		return nil, nil, errTraceDest
	}
	csWrap, err := s.userClient(r)
	if err != nil {
		return nil, nil, err
	}
	tc, err := rbac.NewTraceContext(r.Context(), csWrap.Interface, authSID, req.Namespaces)
	if err != nil {
		return nil, nil, err
	}
	if err := tc.ValidateEndpoints(req.Source, req.Destination); err != nil {
		return nil, nil, err
	}
	cfg, _ := s.userRESTConfig(r)
	eng, err := engine.NewEngineFromREST(csWrap, cfg, tc.Access)
	if err != nil {
		return nil, nil, err
	}
	topo, err := eng.TracePath(r.Context(), req)
	if err != nil {
		return nil, nil, err
	}
	topo = rbac.ApplyTraceSanitization(topo, tc)
	sigma := graphdb.SigmaGraphFromTrace(topo.TraceID, topo.Graph)
	if s.Graph != nil && s.Graph.Enabled() && topo.TraceID != "" {
		go s.syncTraceTopology(topo.TraceID, authSID, topo)
	}
	return topo, sigma, nil
}

func (s *Server) syncTraceTopology(traceID, authSessionID string, topo *engine.TopologyResult) {
	if s.Graph == nil || !s.Graph.Enabled() || topo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = s.Graph.ReplaceTraceGraph(ctx, traceID, authSessionID, topo.Graph)
	_ = s.Graph.UpsertTopology(ctx, traceID, authSessionID, topo)
}

func writeTracePathJSON(w http.ResponseWriter, topo *engine.TopologyResult, sigma *graphdb.SigmaGraph) {
	writeJSON(w, map[string]interface{}{
		"trace_id":       topo.TraceID,
		"source":         topo.Source,
		"destination":    topo.Destination,
		"graph":          topo.Graph,
		"sigma_graph":    sigma,
		"topology":       topo,
		"logical":        topo.Logical,
		"infrastructure": topo.Infrastructure,
		"physical":       topo.Physical,
		"layers":         topo.Layers,
		"edge_states":    topo.EdgeStates,
		"nodes":          topo.Nodes,
		"edges":          topo.Edges,
	})
}
