package sensor

import (
	"testing"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

func TestPacketMatchesPods(t *testing.T) {
	pods := []spcgk8s.PodDetail{{Namespace: "demo", Name: "curl", PodIP: "10.0.0.196"}}
	meta := FlowMetadata{
		"SrcK8S_Namespace": "demo",
		"SrcK8S_Name":      "curl",
		"DstAddr":          "8.8.8.8",
	}
	if !PacketMatchesPods(meta, pods) {
		t.Fatal("expected src pod match")
	}
	meta = FlowMetadata{
		"DstK8S_Namespace": "demo",
		"DstK8S_Name":      "curl",
		"SrcAddr":          "8.8.8.8",
	}
	if !PacketMatchesPods(meta, pods) {
		t.Fatal("expected dst pod match (reply)")
	}
	meta = FlowMetadata{"SrcAddr": "1.2.3.4", "DstAddr": "5.6.7.8"}
	if PacketMatchesPods(meta, pods) {
		t.Fatal("expected no match for unrelated flow")
	}
	meta = FlowMetadata{"SrcAddr": "10.0.0.196", "DstAddr": "8.8.8.8"}
	if !PacketMatchesPods(meta, pods) {
		t.Fatal("expected pod IP src match")
	}
}
