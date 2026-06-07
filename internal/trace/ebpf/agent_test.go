package ebpf

import (
	"net"
	"testing"

	"github.com/netobserv/spcg/internal/trace/engine"
)

func TestCorrelatorActiveAndDrop(t *testing.T) {
	topo := &engine.TopologyResult{
		Edges: []engine.TopologyEdge{
			{ID: "e1", ACLMetadata: "acl-10"},
			{ID: "e2"},
		},
		EdgeStates: map[string]engine.EdgeVerificationState{
			"e1": engine.EdgeTheoryOnly,
			"e2": engine.EdgeTheoryOnly,
		},
	}
	c := NewCorrelator(topo)
	c.Observe(FlowEvent{Hook: HookVethEgress, SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2")})
	if c.Snapshot()["e2"] != engine.EdgeActiveGreen {
		t.Fatalf("states=%v", c.Snapshot())
	}
	c.Observe(FlowEvent{Dropped: true, Hook: HookOVSExecute})
	if c.Snapshot()["e1"] != engine.EdgeDroppedRed {
		t.Fatalf("states=%v", c.Snapshot())
	}
}
