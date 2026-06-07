package rbac

import (
	"testing"

	"github.com/netobserv/spcg/internal/trace/engine"
)

func TestSanitizeTopologyResultStripsPhysical(t *testing.T) {
	topo := &engine.TopologyResult{
		Nodes: []engine.TopologyNode{
			{ID: "pod:a", Label: "a", Layer: engine.LayerLogical, Neo4jLabel: "Pod"},
			{ID: "node:w1", Label: "worker-1", Layer: engine.LayerPhysical, Sensitive: true, Neo4jLabel: "Node"},
		},
		Edges: []engine.TopologyEdge{
			{ID: "e1", From: "pod:a", To: "node:w1", Layer: engine.LayerPhysical},
		},
	}
	out := SanitizeTopologyResult(topo)
	if len(out.Nodes) != 1 || out.Nodes[0].ID != "pod:a" {
		t.Fatalf("nodes=%+v", out.Nodes)
	}
	if len(out.Edges) != 0 {
		t.Fatalf("edges=%+v", out.Edges)
	}
	if len(out.Physical.Nodes) != 0 {
		t.Fatal("physical plane should be cleared")
	}
}
