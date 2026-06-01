package pcap

import (
	"encoding/binary"
	"net"
)

// FrameSummary is a lightweight decode of an Ethernet frame for JSONL export.
type FrameSummary struct {
	Length    int      `json:"length"`
	EtherType string   `json:"ethertype,omitempty"`
	Proto     string   `json:"proto,omitempty"`
	SrcPort   uint16   `json:"src_port,omitempty"`
	DstPort   uint16   `json:"dst_port,omitempty"`
	TCPFlags  []string `json:"tcp_flags,omitempty"`
	DNSQuery  string   `json:"dns_query,omitempty"`
}

// ethernetPayload strips an optional netobserv 16-byte pcap record prefix.
func ethernetPayload(frame []byte) []byte {
	if len(frame) <= 16 {
		return frame
	}
	capLen := binary.LittleEndian.Uint32(frame[8:12])
	if capLen > 0 && int(capLen)+16 <= len(frame) {
		return frame[16 : 16+capLen]
	}
	if len(frame) > 16 && frame[12] == 0 && frame[13] == 0 {
		return frame[16:]
	}
	return frame
}

func ipsFromEthernet(payload []byte) (src, dst string) {
	payload = ethernetPayload(payload)
	if len(payload) < 34 {
		return "", ""
	}
	ethType := binary.BigEndian.Uint16(payload[12:14])
	l3 := payload[14:]
	switch ethType {
	case 0x0800:
		if len(l3) < 20 || (l3[0]>>4) != 4 {
			return "", ""
		}
		return ipv4ToString(l3[12:16]), ipv4ToString(l3[16:20])
	case 0x86DD:
		if len(l3) < 40 || (l3[0]>>4) != 6 {
			return "", ""
		}
		return ipv6ToString(l3[8:24]), ipv6ToString(l3[24:40])
	}
	return "", ""
}

func ipv4ToString(b []byte) string {
	if len(b) != 4 {
		return ""
	}
	return net.IP(b).String()
}

func ipv6ToString(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return net.IP(b).String()
}

func summarizeFrame(b []byte) FrameSummary {
	b = ethernetPayload(b)
	s := FrameSummary{Length: len(b)}
	if len(b) < 14 {
		return s
	}
	ethType := binary.BigEndian.Uint16(b[12:14])
	s.EtherType = hex16(ethType)
	if ethType != 0x0800 && ethType != 0x86DD {
		return s
	}
	l3 := b[14:]
	var proto byte
	var off int
	if ethType == 0x0800 {
		if len(l3) < 20 || (l3[0]>>4) != 4 {
			return s
		}
		ihl := int(l3[0]&0x0f) * 4
		if len(l3) < ihl+4 {
			return s
		}
		proto = l3[9]
		off = 14 + ihl
	} else {
		if len(l3) < 40 || (l3[0]>>4) != 6 {
			return s
		}
		proto = l3[6]
		off = 14 + 40
	}
	if len(b) < off+4 {
		return s
	}
	switch proto {
	case 6:
		s.Proto = "TCP"
		s.SrcPort = be16(b[off : off+2])
		s.DstPort = be16(b[off+2 : off+4])
		if len(b) >= off+14 {
			s.TCPFlags = parseTCPFlagsByte(b[off+13])
		}
	case 17:
		s.Proto = "UDP"
		s.SrcPort = be16(b[off : off+2])
		s.DstPort = be16(b[off+2 : off+4])
		if s.DstPort == 53 || s.SrcPort == 53 {
			s.DNSQuery = parseDNSQuery(b[off+8:])
		}
	case 1, 58:
		if proto == 1 {
			s.Proto = "ICMP"
		} else {
			s.Proto = "ICMPv6"
		}
	}
	return s
}

func parseDNSQuery(payload []byte) string {
	if len(payload) < 13 {
		return ""
	}
	// Skip DNS header (12 bytes), read QNAME labels.
	i := 12
	var labels []string
	for i < len(payload) {
		l := int(payload[i])
		if l == 0 {
			break
		}
		if l > 63 || i+1+l > len(payload) {
			return ""
		}
		labels = append(labels, string(payload[i+1:i+1+l]))
		i += 1 + l
	}
	if len(labels) == 0 {
		return ""
	}
	name := ""
	for j, lb := range labels {
		if j > 0 {
			name += "."
		}
		name += lb
	}
	return name
}

func be16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

func parseTCPFlags(frame []byte) []string {
	frame = ethernetPayload(frame)
	if len(frame) < 34 {
		return nil
	}
	ethType := binary.BigEndian.Uint16(frame[12:14])
	if ethType != 0x0800 {
		return nil
	}
	ihl := int(frame[14]&0x0f) * 4
	if frame[23] != 6 || len(frame) < 14+ihl+14 {
		return nil
	}
	return parseTCPFlagsByte(frame[14+ihl+13])
}

func parseTCPFlagsByte(flags byte) []string {
	var out []string
	if flags&0x02 != 0 {
		out = append(out, "SYN")
	}
	if flags&0x10 != 0 {
		out = append(out, "ACK")
	}
	if flags&0x01 != 0 {
		out = append(out, "FIN")
	}
	if flags&0x04 != 0 {
		out = append(out, "RST")
	}
	if flags&0x08 != 0 {
		out = append(out, "PSH")
	}
	return out
}

func hex16(v uint16) string {
	const hx = "0123456789abcdef"
	return "0x" + string([]byte{hx[v>>12&0xf], hx[v>>8&0xf], hx[v>>4&0xf], hx[v&0xf]})
}
