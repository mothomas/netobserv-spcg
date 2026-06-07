package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/netobserv/spcg/internal/auth"
	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"
	"github.com/netobserv/spcg/internal/trace/probe"
	"k8s.io/client-go/rest"
)

func (s *Server) registerTraceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/trace/discover", s.handleTraceDiscover)
	mux.HandleFunc("/api/v1/trace/start", s.handleTraceStart)
	mux.HandleFunc("/api/v1/trace/graph", s.handleTraceGraph)
	mux.HandleFunc("/api/v1/trace/teardown/", s.handleTraceTeardown)
	s.registerTracePathRoute(mux)
	s.registerTraceCaptureRoutes(mux)
	s.registerTraceProbeRoutes(mux)
}

func (s *Server) handleTraceDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := s.runTraceDiscover(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sigma := graphdb.SigmaGraphFromTrace(resp.TraceID, resp.Graph)
	writeTraceJSON(w, resp, sigma)
}

func (s *Server) handleTraceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resp, err := s.runTraceDiscover(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if resp.TraceID == "" {
		resp.TraceID = uuid.NewString()
		resp.Graph.TraceID = resp.TraceID
	}
	sigma := graphdb.SigmaGraphFromTrace(resp.TraceID, resp.Graph)
	registerTraceSession(resp.TraceID, authSID, *resp, sigma)
	if s.Graph != nil && s.Graph.Enabled() && len(resp.Graph.Nodes) <= 80 {
		go s.syncTraceToGraph(resp.TraceID, authSID, resp.Graph)
	}
	writeTraceJSON(w, resp, sigma)
}

func (s *Server) handleTraceGraph(w http.ResponseWriter, r *http.Request) {
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
	sigma := sess.SigmaGraph
	if sigma == nil {
		sigma = graphdb.SigmaGraphFromTrace(traceID, sess.Response.Graph)
	}
	writeTraceJSON(w, &sess.Response, sigma)
}

func writeTraceJSON(w http.ResponseWriter, resp *trace.DiscoverResponse, sigma *graphdb.SigmaGraph) {
	captureID := ""
	if resp.TraceID != "" {
		captureID = traceCaptureSessionID(resp.TraceID)
	}
	writeJSON(w, map[string]interface{}{
		"trace_id":           resp.TraceID,
		"source":             resp.Source,
		"destination":        resp.Destination,
		"source_pods":        resp.SourcePods,
		"dest_pods":          resp.DestPods,
		"target_pod":         resp.TargetPod,
		"graph":              resp.Graph,
		"resolved":           resp.Resolved,
		"sigma_graph":        sigma,
		"capture_session_id": captureID,
		"capture_active":     captureID != "" && captureStreamActive(captureID),
	})
}

func (s *Server) handleTraceTeardown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/trace/teardown/")
	traceID = strings.Trim(traceID, "/")
	if traceID == "" {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	if !s.requireTraceAccess(w, r, traceID) {
		return
	}
	captureID := stopTraceCapture(traceID)
	if captureID != "" {
		teardownCaptureSession(captureID)
	}
	probe.StopTraceProbe(traceID)
	deleteTraceSession(traceID)
	if s.Graph != nil && s.Graph.Enabled() {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_ = s.Graph.DeleteCapture(ctx, traceID)
	}
	writeJSON(w, map[string]string{"status": "ok", "trace_id": traceID})
}

func (s *Server) syncTraceToGraph(traceID, authSessionID string, g trace.TraceGraph) {
	if s.Graph == nil || !s.Graph.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.Graph.ReplaceTraceGraph(ctx, traceID, authSessionID, g)
}

func (s *Server) runTraceDiscover(r *http.Request) (*trace.DiscoverResponse, error) {
	var req trace.DiscoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	if len(req.Namespaces) == 0 {
		return nil, errTraceNamespaces
	}
	if req.Source.Mode == "" && len(req.Selections) == 0 {
		return nil, errTraceSource
	}
	csWrap, err := s.userClient(r)
	if err != nil {
		return nil, err
	}
	cfg, _ := s.userRESTConfig(r)
	cat, err := trace.OpenCatalog(csWrap.Interface, cfg)
	if err != nil {
		return nil, err
	}
	return cat.Resolve(r.Context(), req)
}

func (s *Server) requireTraceAccess(w http.ResponseWriter, r *http.Request, traceID string) bool {
	authSID, err := s.authSessionID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return false
	}
	if !assertTraceOwner(traceID, authSID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

var (
	errTraceNamespaces = &traceError{"namespaces are required"}
	errTraceSource     = &traceError{"source endpoint is required"}
	errTraceDest       = &traceError{"destination endpoint is required"}
)

type traceError struct{ msg string }

func (e *traceError) Error() string { return e.msg }

// userRESTConfig returns REST config for dynamic CRD discovery (optional).
func (s *Server) userRESTConfig(r *http.Request) (*rest.Config, error) {
	if s.Sessions == nil {
		s.Sessions = auth.NewStore(0)
	}
	sid, mode, bearer, err := auth.ResolveSessionID(r)
	if err != nil {
		return nil, err
	}
	if sid != "" {
		sess, ok := s.Sessions.Get(sid)
		if !ok {
			return nil, fmt.Errorf("session expired or invalid: re-authenticate")
		}
		switch sess.Mode {
		case auth.ModeKubeconfig:
			_, cfg, err := spcgk8s.ClientsetFromKubeconfig(sess.Kubeconfig)
			return cfg, err
		case auth.ModeBearer:
			return spcgk8s.RESTConfigFromBearerToken(sess.Bearer)
		default:
			return nil, fmt.Errorf("unsupported session mode")
		}
	}
	if mode == auth.ModeBearer && bearer != "" {
		return spcgk8s.RESTConfigFromBearerToken(bearer)
	}
	return nil, fmt.Errorf("missing authentication")
}
