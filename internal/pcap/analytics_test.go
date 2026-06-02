package pcap

import "testing"

func TestClassifyPeerIP(t *testing.T) {
	if ClassifyPeerIP("10.96.0.10") != "k8s_service" {
		t.Fatal("service CIDR")
	}
	if ClassifyPeerIP("8.8.8.8") != "external" {
		t.Fatal("external")
	}
}

func TestAnalyzeEventsTCPFailed(t *testing.T) {
	// minimal: empty events
	a := AnalyzeEvents(nil)
	if a.TcpFailedHandshakes != 0 {
		t.Fatal("expected zero")
	}
}
