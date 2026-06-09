package trace

import "testing"

func TestBuildPathOptionsIngressChain(t *testing.T) {
	anchor := "pod/demo/curl"
	nodes := []TraceNode{
		{ID: "external/client-route", Label: "Client", Kind: "external-client"},
		{ID: "route/demo/api-route", Label: "api-route", Kind: "route", Namespace: "demo"},
		{ID: "service/demo/api", Label: "api", Kind: "service-clusterip", Namespace: "demo"},
		{ID: anchor, Label: "curl", Kind: "pod", Namespace: "demo", Tracked: true},
	}
	edges := []TraceEdge{
		{ID: "e1", From: "external/client-route", To: "route/demo/api-route", EdgeType: "ingress"},
		{ID: "e2", From: "route/demo/api-route", To: "service/demo/api", EdgeType: "https"},
		{ID: "e3", From: "service/demo/api", To: anchor, EdgeType: "direct"},
	}
	paths := []PathSummary{{
		Direction: "ingress",
		Resource:  "api-route",
		Namespace: "demo",
		Kind:      "route",
		Status:    "discovered",
	}}

	opts := buildPathOptions(nodes, edges, paths, anchor)
	if len(opts) != 1 {
		t.Fatalf("expected 1 path option, got %d", len(opts))
	}
	if len(opts[0].HopIDs) < 3 {
		t.Fatalf("expected ingress chain hops, got %v", opts[0].HopIDs)
	}
	applyPathRefs(nodes, edges, opts, anchor)
	if nodes[3].Track != "anchor" {
		t.Fatalf("expected anchor track, got %q", nodes[3].Track)
	}
}

func TestApplyPathLayoutSwimlanes(t *testing.T) {
	anchor := "pod/demo/curl"
	nodes := []TraceNode{
		{ID: "external/client-route", Label: "Client", Kind: "external-client", Track: "ingress", PathRefs: []string{"path-0"}},
		{ID: anchor, Label: "curl", Kind: "pod", Track: "anchor", Tracked: true},
		{ID: "egressip/prod", Label: "prod", Kind: "egressip", Track: "egress", PathRefs: []string{"path-1"}},
	}
	opts := []PathOption{
		{ID: "path-0", Direction: PathIngress, HopIDs: []string{"external/client-route", anchor}},
		{ID: "path-1", Direction: PathEgress, HopIDs: []string{anchor, "egressip/prod"}},
	}
	g, lanes := applyPathLayout(nodes, opts, anchor)
	if len(lanes) != 4 {
		t.Fatalf("expected 4 swimlanes, got %d", len(lanes))
	}
	if g.Height < 400 {
		t.Fatalf("expected tall swimlane canvas, got height %v", g.Height)
	}
}
