package graphdb

import (
	"testing"

	"github.com/netobserv/spcg/internal/trace"
)

func TestSigmaGraphFromTrace(t *testing.T) {
	g := trace.TraceGraph{
		TraceID: "t1",
		Nodes: []trace.TraceNode{
			{ID: "pod/demo/curl", Label: "curl", Kind: "pod", Tracked: true, X: 100, Y: 50, Width: 148, Height: 72},
			{ID: "service/demo/api", Label: "api", Kind: "service-clusterip", X: 0, Y: 50, Width: 148, Height: 72},
		},
		Edges: []trace.TraceEdge{
			{ID: "e1", From: "service/demo/api", To: "pod/demo/curl", EdgeType: "direct", Primary: true},
		},
	}
	sg := SigmaGraphFromTrace("t1", g)
	if len(sg.Nodes) != 2 {
		t.Fatalf("nodes: %d", len(sg.Nodes))
	}
	if !sg.Nodes[0].Tracked && !sg.Nodes[1].Tracked {
		t.Fatal("expected tracked pod")
	}
	if len(sg.Edges) != 1 || sg.Edges[0].EdgeType != "direct" {
		t.Fatalf("edges: %+v", sg.Edges)
	}
}
