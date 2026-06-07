package probe

// EdgePaintState is live verification paint on a predicted trace edge.
type EdgePaintState string

const (
	EdgeTheoryOnly  EdgePaintState = "THEORY_ONLY"
	EdgeActiveGreen EdgePaintState = "ACTIVE_GREEN"
	EdgeDroppedRed  EdgePaintState = "DROPPED_RED"
)

// AttachInterface is a pod network attachment usable for probe egress.
type AttachInterface struct {
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
	CNI     string `json:"cni,omitempty"`
}

// FireRequest starts a painted probe from a source pod interface.
type FireRequest struct {
	TraceID   string `json:"trace_id"`
	Interface string `json:"interface"` // "default" or Multus net name
	Simulate  bool   `json:"simulate,omitempty"`
	DemoDrop  bool   `json:"demo_drop,omitempty"`
}

// FireResponse acknowledges a probe session and paint token.
type FireResponse struct {
	ProbeID            string `json:"probe_id"`
	TraceID            string `json:"trace_id"`
	PaintToken         string `json:"paint_token"`
	ICMPID             uint16 `json:"icmp_id"`
	Interface          string `json:"interface"`
	Mode               string `json:"mode"` // simulate | capture | live
	PrimaryEdges       int    `json:"primary_edges"`
	CaptureLinked      bool   `json:"capture_linked,omitempty"`
	CaptureAutoStarted bool   `json:"capture_auto_started,omitempty"`
}

// ProbeEvent is streamed to the UI while a probe is active.
type ProbeEvent struct {
	Type       string         `json:"type"` // probe_started, edge_update, probe_finished, error
	TraceID    string         `json:"trace_id"`
	ProbeID    string         `json:"probe_id,omitempty"`
	EdgeID     string         `json:"edge_id,omitempty"`
	State      EdgePaintState `json:"state,omitempty"`
	Hook       string         `json:"hook,omitempty"`
	Seq        int            `json:"seq,omitempty"`
	Message    string         `json:"message,omitempty"`
	DropReason string         `json:"drop_reason,omitempty"`
	Verified   int            `json:"verified,omitempty"`
	Total      int            `json:"total,omitempty"`
}
