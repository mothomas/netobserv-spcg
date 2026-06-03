package pcap

import (
	"testing"
	"time"
)

func TestConversationSequenceMergesDirections(t *testing.T) {
	base := time.Unix(0, 0)
	topo := FlowTopology{
		Edges: []TopologyEdge{{ID: "a->b", From: "a", To: "b"}},
		EdgeDetail: map[string]EdgeDetail{
			"a->b": {
				Sequence: []SequenceStep{
					{RelUs: 0, AtUs: base.UnixMicro(), Direction: "forward", Phase: "start", Label: "Start · SYN"},
					{RelUs: 100, AtUs: base.Add(100 * time.Microsecond).UnixMicro(), Direction: "forward", Phase: "reply", Label: "Reply · ACK"},
				},
			},
			"b->a": {
				Sequence: []SequenceStep{
					{RelUs: 0, AtUs: base.Add(50 * time.Microsecond).UnixMicro(), Direction: "forward", Phase: "reply", Label: "Reply · SYN+ACK"},
				},
			},
		},
	}
	seq := ConversationSequence(topo, "a->b")
	if len(seq) != 3 {
		t.Fatalf("len=%d want 3", len(seq))
	}
	if seq[0].Phase != "start" || seq[1].Label != "Reply · SYN+ACK" || seq[2].Label != "Reply · ACK" {
		t.Fatalf("order=%v", seq)
	}
	if seq[1].Direction != "reverse" {
		t.Fatalf("middle direction=%s", seq[1].Direction)
	}
}

func TestSequencePhase(t *testing.T) {
	if sequencePhase([]string{"SYN"}) != "start" {
		t.Fatal("syn")
	}
	if sequencePhase([]string{"SYN", "ACK"}) != "reply" {
		t.Fatal("syn ack")
	}
	if sequencePhase([]string{"RST"}) != "close" {
		t.Fatal("rst")
	}
}
