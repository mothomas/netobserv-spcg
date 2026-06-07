package engine

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseOpenFlowDump(t *testing.T) {
	dump := ` cookie=0xabc, duration=1.0s, table=10, n_packets=3, n_bytes=120, priority=100, ip actions=set_field:4096->tun_id,output:2
 cookie=0xdead, duration=2.0s, table=0, priority=0 actions=drop`
	rules := parseOpenFlowDump("worker-1", "br-int", dump)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Cookie != "0xabc" {
		t.Fatalf("cookie=%q", rules[0].Cookie)
	}
	if rules[0].GeneveVNI != "4096" {
		t.Fatalf("vni=%q", rules[0].GeneveVNI)
	}
	if !rules[1].Terminates {
		t.Fatal("expected drop rule to terminate")
	}
}

func TestEnrichNADConfigOVNOverlay(t *testing.T) {
	att := MultusAttachment{}
	enrichNADConfig(&att, unstructuredNAD(`{"type":"ovn-k8s-cni-overlay","topology":"localnet","vlanID":200,"subnets":"10.20.0.0/24"}`))
	if att.Topology != "localnet" || att.VLANID != "200" {
		t.Fatalf("unexpected overlay config: %+v", att)
	}
}

func TestEnrichNADConfigHostBypass(t *testing.T) {
	att := MultusAttachment{}
	enrichNADConfig(&att, unstructuredNAD(`{"type":"macvlan","master":"eth1.200"}`))
	if !att.HostBypass || att.HostIface != "eth1.200" {
		t.Fatalf("unexpected macvlan config: %+v", att)
	}
}

func unstructuredNAD(config string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"config": config}}}
}
