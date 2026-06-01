package pcap

import "testing"

func TestBuildFlowTopologyCapturePodNoIPs(t *testing.T) {
	tracked := []TrackedPod{{Namespace: "demo", Name: "curl"}}
	ev := FlowEvent{
		CapturePod: "demo/curl",
		FlowMeta:   map[string]interface{}{"SrcAddr": "10.0.0.50", "DstAddr": "8.8.8.8", "Proto": "UDP"},
	}
	topo := BuildFlowTopology([]FlowEvent{ev}, tracked)
	if len(topo.Edges) != 1 {
		t.Fatalf("edges=%d want 1", len(topo.Edges))
	}
	if topo.Edges[0].From != "demo/curl" {
		t.Fatalf("from=%s", topo.Edges[0].From)
	}
	filtered := FilterTopologyToSelection(topo, trackedNodeIDs(tracked))
	if len(filtered.Edges) != 1 {
		t.Fatalf("filtered edges=%d", len(filtered.Edges))
	}
}

func TestResolveFlowEndpointsPodToExternal(t *testing.T) {
	tracked := []TrackedPod{
		{Namespace: "demo", Name: "curl", PodIP: "10.0.0.196", PodIPs: []string{"10.0.0.196"}},
	}
	ev := FlowEvent{
		CapturePod: "demo/curl",
		FlowMeta:   map[string]interface{}{"SrcAddr": "10.0.0.196", "DstAddr": "8.8.8.8"},
	}
	from, to := resolveFlowEndpoints(ev, tracked)
	if from.ID != "demo/curl" {
		t.Fatalf("from=%q want demo/curl", from.ID)
	}
	if to.ID != "ext/8.8.8.8" {
		t.Fatalf("to=%q want ext/8.8.8.8", to.ID)
	}
	topo := BuildFlowTopology([]FlowEvent{ev}, tracked)
	if len(topo.Edges) != 1 {
		t.Fatalf("edges=%d want 1", len(topo.Edges))
	}
}

func TestOrientCapturePodWithoutIPMap(t *testing.T) {
	cap := TopologyNode{ID: "demo/curl", Label: "curl", Kind: "Pod", Namespace: "demo", Pod: "curl"}
	from := externalNode("10.0.0.50")
	to := externalNode("8.8.8.8")
	gotFrom, gotTo := orientToCapturePod(from, to, cap, "10.0.0.50", "8.8.8.8", map[string]podRef{})
	if gotFrom.ID != "demo/curl" {
		t.Fatalf("from=%q want demo/curl", gotFrom.ID)
	}
	if gotTo.ID != "ext/8.8.8.8" {
		t.Fatalf("to=%q want ext/8.8.8.8", gotTo.ID)
	}
}

func TestBuildCaptureSummary(t *testing.T) {
	tracked := []TrackedPod{{Namespace: "demo", Name: "curl", PodIPs: []string{"10.0.0.1"}}}
	events := []FlowEvent{
		{FlowMeta: map[string]interface{}{"SrcAddr": "10.0.0.1", "DstAddr": "1.1.1.1", "Proto": "ICMP"}},
		{FlowMeta: map[string]interface{}{"SrcAddr": "1.1.1.1", "DstAddr": "10.0.0.1", "Proto": "ICMP"}},
	}
	sum := BuildCaptureSummary(events, tracked)
	if sum.EventCount != 2 {
		t.Fatalf("events=%d", sum.EventCount)
	}
	if sum.FlowEdges == 0 {
		t.Fatal("expected flow edges")
	}
}
