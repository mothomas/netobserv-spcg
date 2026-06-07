package probe

import (
	"encoding/binary"
	"testing"

	"github.com/netobserv/spcg/internal/pcap"
	"github.com/netobserv/spcg/internal/trace"
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

func TestGraphCorrelatorDemoDrop(t *testing.T) {
	g := trace.TraceGraph{
		Nodes: []trace.TraceNode{{ID: "a", Rank: 0}, {ID: "b", Rank: 1}},
		Edges: []trace.TraceEdge{{ID: "e1", From: "a", To: "b", Primary: true}},
	}
	c := NewGraphCorrelator(g)
	if c.NextEdgeID() != "e1" {
		t.Fatalf("next=%s", c.NextEdgeID())
	}
	if !c.MarkDropOnEdge("e1") {
		t.Fatal("mark drop failed")
	}
	if c.Snapshot()["e1"] != EdgeDroppedRed {
		t.Fatalf("state=%v", c.Snapshot()["e1"])
	}
}

func TestMatchPaintPacketICMPFrame(t *testing.T) {
	const want = uint16(0xa3f2)
	frame := buildICMPEchoFrame(want)
	if !MatchPaintPacket(frame, nil, want) {
		t.Fatal("expected icmp frame match")
	}
	if MatchPaintPacket(frame, nil, want+1) {
		t.Fatal("expected mismatch")
	}
	id, ok := pcap.ICMPIdentifier(frame)
	if !ok || id != want {
		t.Fatalf("parse id=%d ok=%v", id, ok)
	}
}

func buildICMPEchoFrame(icmpID uint16) []byte {
	// Minimal IPv4 ICMP echo frame (no pcap prefix).
	frame := make([]byte, 14+20+8)
	// Ethernet
	frame[12] = 0x08
	frame[13] = 0x00
	// IPv4 header
	frame[14] = 0x45
	frame[23] = 1 // ICMP
	// ICMP echo request
	frame[14+20] = 8
	binary.BigEndian.PutUint16(frame[14+20+4:14+20+6], icmpID)
	return frame
}
