package sensor

import (
	"encoding/binary"
	"testing"

	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbpacket"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestExtractPacketBytesFromPBStripsPcapHeader(t *testing.T) {
	raw := make([]byte, 16+14+20)
	binary.LittleEndian.PutUint32(raw[8:12], 34)
	raw[16+12] = 0x08
	raw[16+13] = 0x00
	raw[16+14] = 0x45
	copy(raw[16+14+12:], []byte{10, 0, 0, 1})
	pkt := &pbpacket.Packet{Pcap: &anypb.Any{Value: raw}}
	got, err := ExtractPacketBytesFromPB(pkt)
	if err != nil || len(got) != 34 {
		t.Fatalf("got len=%d err=%v", len(got), err)
	}
	if got[12] != 0x08 || got[13] != 0x00 {
		t.Fatalf("expected ethertype IPv4 in stripped frame")
	}
}

func TestFlowMetadataFromEthernetIPv4(t *testing.T) {
	raw := make([]byte, 16+14+20)
	binary.LittleEndian.PutUint32(raw[8:12], 34)
	raw[16+12] = 0x08
	raw[16+13] = 0x00
	raw[16+14] = 0x45
	copy(raw[16+14+12:], []byte{10, 0, 0, 134})
	copy(raw[16+14+16:], []byte{8, 8, 8, 8})
	eth := pcapRecordPayload(raw)
	m := FlowMetadataFromFrame(eth)
	if m["SrcAddr"] != "10.0.0.134" || m["DstAddr"] != "8.8.8.8" {
		t.Fatalf("got %+v", m)
	}
}

func TestFlowMetadataFromEthernetIPv6(t *testing.T) {
	raw := make([]byte, 16+14+40)
	binary.LittleEndian.PutUint32(raw[8:12], 54)
	raw[16+12] = 0x86
	raw[16+13] = 0xdd
	raw[16+14] = 0x60 // version 6
	copy(raw[16+14+8:], []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	})
	copy(raw[16+14+24:], []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
	})
	eth := pcapRecordPayload(raw)
	m := FlowMetadataFromFrame(eth)
	if m["SrcAddr"] != "2001:db8::1" || m["DstAddr"] != "2001:db8::2" {
		t.Fatalf("got %+v", m)
	}
}
