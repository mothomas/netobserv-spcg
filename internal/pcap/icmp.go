package pcap

import "encoding/binary"

// ICMPIdentifier returns the ICMP echo id field from an Ethernet frame, if present.
func ICMPIdentifier(frame []byte) (uint16, bool) {
	frame = ethernetPayload(frame)
	if len(frame) < 14 {
		return 0, false
	}
	ethType := binary.BigEndian.Uint16(frame[12:14])
	if ethType != 0x0800 {
		return 0, false
	}
	l3 := frame[14:]
	if len(l3) < 28 || (l3[0]>>4) != 4 || l3[9] != 1 {
		return 0, false
	}
	ihl := int(l3[0]&0x0f) * 4
	if len(l3) < ihl+8 {
		return 0, false
	}
	icmpOff := ihl
	typ := l3[icmpOff]
	if typ != 8 && typ != 0 {
		return 0, false
	}
	id := binary.BigEndian.Uint16(l3[icmpOff+4 : icmpOff+6])
	return id, true
}

// DropCause returns the latest drop cause from netobserv flow metadata, if any.
func DropCause(meta map[string]interface{}) string {
	if meta == nil {
		return ""
	}
	for _, key := range []string{"PktDropLatestDropCause", "PktDropLatestState"} {
		if v, ok := meta[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
