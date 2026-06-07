package probe

import (
	"testing"

	"github.com/netobserv/spcg/internal/trace"
	corev1 "k8s.io/api/core/v1"
)

func TestPaintTokenStable(t *testing.T) {
	a, idA := PaintToken("trace-abc")
	b, idB := PaintToken("trace-abc")
	if a != b || idA != idB || idA == 0 {
		t.Fatalf("token=%s id=%d", a, idA)
	}
}

func TestGraphCorrelatorAdvancesPrimaryEdges(t *testing.T) {
	g := trace.TraceGraph{
		Nodes: []trace.TraceNode{
			{ID: "a", Rank: 0},
			{ID: "b", Rank: 1},
			{ID: "c", Rank: 2},
		},
		Edges: []trace.TraceEdge{
			{ID: "e1", From: "a", To: "b", Primary: true},
			{ID: "e2", From: "b", To: "c", Primary: true},
			{ID: "ctx", From: "a", To: "c", Primary: false},
		},
	}
	c := NewGraphCorrelator(g)
	if c.PrimaryCount() != 2 {
		t.Fatalf("primary count: %d", c.PrimaryCount())
	}
	id1, ok := c.Advance("veth_egress")
	if !ok || id1 != "e1" {
		t.Fatalf("first advance: %s ok=%v", id1, ok)
	}
	id2, ok := c.Advance("ovs_execute_actions")
	if !ok || id2 != "e2" {
		t.Fatalf("second advance: %s ok=%v", id2, ok)
	}
	if _, ok := c.Advance("physical_egress"); ok {
		t.Fatal("expected no more edges")
	}
	snap := c.Snapshot()
	if snap["e1"] != EdgeActiveGreen || snap["e2"] != EdgeActiveGreen {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestInterfacesFromPodMultus(t *testing.T) {
	p := &corev1.Pod{}
	p.Annotations = map[string]string{
		networksStatusAnnotation: `[{"name":"macvlan-conf","interface":"net1","default":false}]`,
	}
	ifaces := interfacesFromPod(p)
	if len(ifaces) < 2 {
		t.Fatalf("expected default + multus, got %+v", ifaces)
	}
	found := false
	for _, iface := range ifaces {
		if iface.Name == "macvlan-conf" && iface.CNI == "multus" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ifaces: %+v", ifaces)
	}
}
