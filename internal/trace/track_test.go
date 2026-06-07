package trace

import "testing"

func TestApplyLayoutDualTracks(t *testing.T) {
	nodes := []TraceNode{
		{ID: "lb", Label: "vip", Kind: "loadbalancer-external", Rank: 1},
		{ID: "svc", Label: "svc", Kind: "service-clusterip", Rank: 2},
		{ID: "src", Label: "pod", Kind: "pod", Tracked: true, Rank: 0},
		{ID: "eg", Label: "egress", Kind: "egressip", Rank: 4},
		{ID: "dst", Label: "8.8.8.8", Kind: "external", Tracked: true, Rank: 5},
	}
	edges := []TraceEdge{
		{From: "lb", To: "svc", EdgeType: "ingress", Primary: true},
		{From: "svc", To: "src", EdgeType: "ingress", Primary: true},
		{From: "src", To: "eg", EdgeType: "egress", Primary: true},
		{From: "eg", To: "dst", EdgeType: "egress", Primary: true},
	}
	paths := []PathSummary{
		{Direction: "ingress", Resource: "svc", Kind: "service-clusterip", Status: "discovered"},
		{Direction: "egress", Resource: "egress", Kind: "egressip", Status: "discovered"},
	}
	g, lanes := applyLayout(nodes, edges, paths)
	if len(lanes) < 3 {
		t.Fatalf("expected swimlanes, got %d", len(lanes))
	}
	byTrack := map[string]float64{}
	for _, n := range g.Nodes {
		byTrack[n.Track] = n.Y
	}
	if byTrack[TrackIngress] >= byTrack[TrackAnchor] {
		t.Fatalf("ingress Y should be above anchor: %+v", byTrack)
	}
	if byTrack[TrackAnchor] >= byTrack[TrackEgress] {
		t.Fatalf("anchor Y should be above egress: %+v", byTrack)
	}
}
