package sensor

import (
	"encoding/binary"
	"net"

	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbpacket"
)

// ExtractPacketBytesFromPB returns the Ethernet frame (no netobserv per-record pcap header).
func ExtractPacketBytesFromPB(pkt *pbpacket.Packet) ([]byte, error) {
	if pkt == nil || pkt.Pcap == nil {
		return nil, nil
	}
	return pcapRecordPayload(pkt.Pcap.Value), nil
}

// FlowMetadataFromFrame builds minimal flow metadata from an Ethernet frame.
func FlowMetadataFromFrame(ethFrame []byte) FlowMetadata {
	src, dst := ipAddrsFromEthernet(ethFrame)
	if src == "" && dst == "" {
		return nil
	}
	m := make(FlowMetadata)
	if src != "" {
		m["SrcAddr"] = src
	}
	if dst != "" {
		m["DstAddr"] = dst
	}
	if proto, sp, dp := l4FromEthernet(ethFrame); proto != "" {
		m["Proto"] = proto
		if sp > 0 {
			m["SrcPort"] = sp
		}
		if dp > 0 {
			m["DstPort"] = dp
		}
	}
	return m
}

func l4FromEthernet(ethFrame []byte) (proto string, srcPort, dstPort uint16) {
	if len(ethFrame) < 34 {
		return "", 0, 0
	}
	ethType := binary.BigEndian.Uint16(ethFrame[12:14])
	if ethType != 0x0800 {
		return "", 0, 0
	}
	ihl := int(ethFrame[14]&0x0f) * 4
	if len(ethFrame) < 14+ihl+4 {
		return "", 0, 0
	}
	switch ethFrame[23] {
	case 6:
		off := 14 + ihl
		return "TCP", binary.BigEndian.Uint16(ethFrame[off : off+2]), binary.BigEndian.Uint16(ethFrame[off+2 : off+4])
	case 17:
		off := 14 + ihl
		return "UDP", binary.BigEndian.Uint16(ethFrame[off : off+2]), binary.BigEndian.Uint16(ethFrame[off+2 : off+4])
	case 1:
		return "ICMP", 0, 0
	}
	return "", 0, 0
}

func ipAddrsFromEthernet(payload []byte) (src, dst string) {
	if len(payload) < 34 {
		return "", ""
	}
	ethType := binary.BigEndian.Uint16(payload[12:14])
	if ethType != 0x0800 && ethType != 0x86DD {
		return "", ""
	}
	l3 := payload[14:]
	if ethType == 0x0800 {
		if len(l3) < 20 || (l3[0]>>4) != 4 {
			return "", ""
		}
		src = net.IP(l3[12:16]).String()
		dst = net.IP(l3[16:20]).String()
		return src, dst
	}
	if len(l3) < 40 || (l3[0]>>4) != 6 {
		return "", ""
	}
	src = net.IP(l3[8:24]).String()
	dst = net.IP(l3[24:40]).String()
	return src, dst
}

// pcapRecordPayload strips the 16-byte pcap per-record header netobserv prepends on the wire.
func pcapRecordPayload(frame []byte) []byte {
	if len(frame) <= 16 {
		return frame
	}
	capLen := binary.LittleEndian.Uint32(frame[8:12])
	if capLen > 0 && int(capLen)+16 <= len(frame) {
		return frame[16 : 16+capLen]
	}
	return frame[16:]
}
