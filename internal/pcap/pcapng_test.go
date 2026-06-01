package pcap

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestEncodePCAPngStartsWithSHB(t *testing.T) {
	// Minimal Ethernet-like payload (not a valid frame, only for header test).
	payload := []byte{0xa2, 0xc0, 0x3b, 0x77, 0x71, 0xf0, 0x9a, 0x68}
	out := EncodePCAPng([]frameRecord{{Data: payload, At: time.Unix(1, 0)}})
	if len(out) < 4 {
		t.Fatalf("output too short: %d", len(out))
	}
	if binary.LittleEndian.Uint32(out[:4]) != blockTypeSHB {
		t.Fatalf("expected SHB magic 0x0a0d0d0a, got %08x", binary.LittleEndian.Uint32(out[:4]))
	}
}

func TestIsPCAPContainer(t *testing.T) {
	raw := []byte{0xa2, 0xc0, 0x3b, 0x77}
	if IsPCAPContainer(raw) {
		t.Fatal("raw ethernet should not be detected as pcap container")
	}
	ng := EncodePCAPng([]frameRecord{{Data: raw, At: time.Now()}})
	if !IsPCAPContainer(ng) {
		t.Fatal("encoded output should be detected as pcap container")
	}
}
