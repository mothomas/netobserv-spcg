package pcap

import "testing"

func TestICMPIdentifierEcho(t *testing.T) {
	frame := make([]byte, 14+20+8)
	frame[12] = 0x08
	frame[13] = 0x00
	frame[14] = 0x45
	frame[23] = 1
	frame[34] = 8
	frame[38] = 0xa3
	frame[39] = 0xf2
	id, ok := ICMPIdentifier(frame)
	if !ok || id != 0xa3f2 {
		t.Fatalf("id=%#x ok=%v", id, ok)
	}
}
