package pcap

import (
	"testing"
	"time"
)

func TestLimitFlowTopologyCapsExternals(t *testing.T) {
	topo := FlowTopology{
		Nodes: []TopologyNode{
			{ID: "ns/pod-a", Label: "pod-a", Kind: "Pod", Namespace: "ns"},
		},
		Edges: nil,
	}
	for i := 0; i < 200; i++ {
		ext := TopologyNode{ID: "ext/1.2.3." + itoaInt(i), Label: "1.2.3." + itoaInt(i), Kind: "External"}
		topo.Nodes = append(topo.Nodes, ext)
		topo.Edges = append(topo.Edges, TopologyEdge{
			ID: edgeID("ns/pod-a", ext.ID), From: "ns/pod-a", To: ext.ID,
			Count: 1, Bytes: uint64(1000 - i), Packets: 1, Health: "healthy",
		})
	}
	tracked := []TrackedPod{{Namespace: "ns", Name: "pod-a"}}
	out, capped := LimitFlowTopology(topo, tracked, 20, 15)
	if !capped {
		t.Fatal("expected capped")
	}
	if len(out.Edges) > 15 {
		t.Fatalf("edges=%d", len(out.Edges))
	}
	if len(out.Nodes) > 20 {
		t.Fatalf("nodes=%d", len(out.Nodes))
	}
}

func TestSampleEventsForGraphKeepsTail(t *testing.T) {
	events := make([]FlowEvent, 10)
	for i := range events {
		events[i] = FlowEvent{At: time.Unix(int64(i), 0)}
	}
	out := SampleEventsForGraph(events, 3)
	if len(out) != 3 || out[0].At.Unix() != 7 {
		t.Fatalf("got %d events start=%d", len(out), out[0].At.Unix())
	}
}
