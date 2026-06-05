package trace

import spcgk8s "github.com/netobserv/spcg/internal/k8s"

// HopStatus is evidence state for a custody hop (live correlation fills these later).
type HopStatus string

const (
	HopPending HopStatus = "pending"
	HopFlowing HopStatus = "flowing"
	HopSeen    HopStatus = "seen"
	HopDropped HopStatus = "dropped"
)

// TraceNode is an infrastructure vertex in the packet-cop graph.
type TraceNode struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Kind      string  `json:"kind"`
	Namespace string  `json:"namespace,omitempty"`
	Rank      int     `json:"rank"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Tracked   bool    `json:"tracked"`
	Status    string  `json:"status,omitempty"`
	Detail    string  `json:"detail,omitempty"`
}

// TraceEdge connects infrastructure nodes (path skeleton).
type TraceEdge struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	EdgeType string `json:"edge_type"` // direct, ingress, egress, policy, host
	Primary  bool   `json:"primary,omitempty"`
	Drop     bool   `json:"drop,omitempty"`
	Label    string `json:"label,omitempty"`
}

// PathSummary is a discovered ingress/egress surface for the UI table.
type PathSummary struct {
	Direction  string `json:"direction"` // ingress, egress, host
	Resource   string `json:"resource"`
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"`
	Status     string `json:"status"` // discovered, out_of_scope
	Detail     string `json:"detail,omitempty"`
}

// TraceGraph is the infrastructure skeleton returned to the UI.
type TraceGraph struct {
	TraceID    string        `json:"trace_id,omitempty"`
	Nodes      []TraceNode   `json:"nodes"`
	Edges      []TraceEdge   `json:"edges"`
	Paths      []PathSummary `json:"paths"`
	Namespaces []string      `json:"namespaces"`
	Lanes      []TraceLane   `json:"lanes,omitempty"`
	Width      float64       `json:"width"`
	Height     float64       `json:"height"`
}

// TraceLane labels a ranked column in the cop timeline.
type TraceLane struct {
	Label string  `json:"label"`
	Rank  int     `json:"rank"`
	X     float64 `json:"x"`
	Width float64 `json:"width"`
}

// DiscoverRequest mirrors capture selection (namespace-scoped).
type DiscoverRequest struct {
	Namespaces  []string                  `json:"namespaces"`
	Selections  []spcgk8s.CaptureSelection  `json:"selections"`
	TraceID     string                    `json:"trace_id,omitempty"`
}

// DiscoverResponse is returned from discover/start trace APIs.
type DiscoverResponse struct {
	TraceID    string                  `json:"trace_id"`
	TargetPod  spcgk8s.PodDetail       `json:"target_pod"`
	Graph    TraceGraph              `json:"graph"`
	Resolved spcgk8s.ResolvedCapture `json:"resolved,omitempty"`
}
