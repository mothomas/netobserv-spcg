package pcap

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

// PacketAnalytics holds deep stats from per-packet decode (short PCAP windows).
type PacketAnalytics struct {
	TcpSyn               int            `json:"tcp_syn"`
	TcpSynAck            int            `json:"tcp_syn_ack"`
	TcpRst               int            `json:"tcp_rst"`
	TcpFin               int            `json:"tcp_fin"`
	TcpFailedHandshakes  int            `json:"tcp_failed_handshakes"`
	ICMP                 map[string]int `json:"icmp"`
	PeerClasses          map[string]int `json:"peer_classes"`
	TLSSNI               []SNIStat      `json:"tls_sni"`
	DNSFailures          map[string]int `json:"dns_failures"`
	DNSResponses         int            `json:"dns_responses"`
	TimeBuckets          []TimeBucket   `json:"time_buckets"`
	PeakBucketPackets    int            `json:"peak_bucket_packets"`
	PeakBucketBitsPerSec uint64         `json:"peak_bucket_bits_per_sec"`
}

type SNIStat struct {
	Host  string `json:"host"`
	Count int    `json:"count"`
}

type TimeBucket struct {
	OffsetMs int    `json:"offset_ms"`
	Packets  int    `json:"packets"`
	Bytes    uint64 `json:"bytes"`
}

type tcpFlowTrack struct {
	syn       bool
	synAck    bool
}

const bucketMs = 100

// AnalyzeEvents scans frames for TCP/DNS/TLS/ICMP signals.
func AnalyzeEvents(events []FlowEvent) PacketAnalytics {
	out := PacketAnalytics{
		ICMP:        map[string]int{},
		PeerClasses: map[string]int{},
		DNSFailures: map[string]int{},
	}
	if len(events) == 0 {
		return out
	}

	first := events[0].At
	var last time.Time
	flows := map[string]*tcpFlowTrack{}
	buckets := map[int]*TimeBucket{}
	sniCounts := map[string]int{}

	for _, ev := range events {
		if ev.At.After(last) {
			last = ev.At
		}
		frame := ethernetPayload(ev.Frame)
		fs := mergeFrameMeta(summarizeFrame(frame), ev.FlowMeta)

		offMs := int(ev.At.Sub(first).Milliseconds())
		bi := offMs / bucketMs
		b, ok := buckets[bi]
		if !ok {
			b = &TimeBucket{OffsetMs: bi * bucketMs}
			buckets[bi] = b
		}
		b.Packets++
		if len(frame) > 0 {
			b.Bytes += uint64(len(frame))
		}

		srcIP := flowString(ev.FlowMeta, "SrcAddr")
		dstIP := flowString(ev.FlowMeta, "DstAddr")
		if srcIP == "" || dstIP == "" {
			fSrc, fDst := ipsFromEthernet(frame)
			if srcIP == "" {
				srcIP = fSrc
			}
			if dstIP == "" {
				dstIP = fDst
			}
		}
		for _, ip := range []string{srcIP, dstIP} {
			if ip != "" {
				out.PeerClasses[ClassifyPeerIP(ip)]++
			}
		}

		if fs.Proto == "TCP" {
			trackTCPFlows(flows, frame, fs, &out)
		}
		if icmpType := parseICMP(frame); icmpType != "" {
			out.ICMP[icmpType]++
		}
		if host := parseTLSSNI(frame); host != "" {
			sniCounts[host]++
		}
		if rcode, isResp := parseDNSResponse(frame); isResp {
			out.DNSResponses++
			if rcode != "NOERROR" && rcode != "" {
				out.DNSFailures[rcode]++
			}
		}
	}

	out.TcpFailedHandshakes = countFailedTCP(flows)
	out.TimeBuckets = sortedBuckets(buckets)
	out.TLSSNI = topSNI(sniCounts, 8)
	for _, b := range out.TimeBuckets {
		if b.Packets > out.PeakBucketPackets {
			out.PeakBucketPackets = b.Packets
			// bits in 100ms window
			out.PeakBucketBitsPerSec = b.Bytes * 8 * 10
		}
	}
	if !last.After(first) {
		last = first
	}
	_ = last
	return out
}

func trackTCPFlows(flows map[string]*tcpFlowTrack, frame []byte, fs FrameSummary, out *PacketAnalytics) {
	for _, f := range fs.TCPFlags {
		switch f {
		case "SYN":
			out.TcpSyn++
		case "ACK":
			// counted with SYN below
		case "RST":
			out.TcpRst++
		case "FIN":
			out.TcpFin++
		}
	}
	src, dst := ipsFromEthernet(frame)
	if src == "" || dst == "" || fs.SrcPort == 0 {
		return
	}
	k := fmt.Sprintf("%s:%d>%s:%d", src, fs.SrcPort, dst, fs.DstPort)
	rev := fmt.Sprintf("%s:%d>%s:%d", dst, fs.DstPort, src, fs.SrcPort)
	st, ok := flows[k]
	if !ok {
		st = &tcpFlowTrack{}
		flows[k] = st
	}
	hasSyn := flagHas(fs.TCPFlags, "SYN")
	hasAck := flagHas(fs.TCPFlags, "ACK")
	if hasSyn && !hasAck {
		st.syn = true
	}
	if hasSyn && hasAck {
		out.TcpSynAck++
		if rs, ok := flows[rev]; ok {
			rs.synAck = true
		} else {
			flows[rev] = &tcpFlowTrack{synAck: true}
		}
	}
	if hasSyn && hasAck {
		st.synAck = true
	}
}

func countFailedTCP(flows map[string]*tcpFlowTrack) int {
	n := 0
	for _, st := range flows {
		if st.syn && !st.synAck {
			n++
		}
	}
	return n
}

func flagHas(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// ClassifyPeerIP buckets destinations for customer-facing summaries.
func ClassifyPeerIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		if strings.Contains(ip, ".svc.cluster.local") || strings.Contains(ip, "cluster.local") {
			return "cluster_dns"
		}
		return "unknown"
	}
	if parsed.IsLoopback() {
		return "loopback"
	}
	if parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() {
		return "link_local"
	}
	if v4 := parsed.To4(); v4 != nil {
		if v4[0] == 10 && v4[1] == 96 {
			return "k8s_service"
		}
		if v4[0] == 10 || (v4[0] == 172 && v4[1]&0xf0 == 16) || (v4[0] == 192 && v4[1] == 168) {
			return "k8s_pod"
		}
		return "external"
	}
	if parsed.IsPrivate() {
		return "k8s_pod"
	}
	return "external"
}

func parseICMP(frame []byte) string {
	frame = ethernetPayload(frame)
	if len(frame) < 14 {
		return ""
	}
	eth := binary.BigEndian.Uint16(frame[12:14])
	if eth != 0x0800 {
		if eth == 0x86DD && len(frame) >= 54 {
			return parseICMPv6(frame[14:])
		}
		return ""
	}
	l3 := frame[14:]
	if len(l3) < 20 || (l3[0]>>4) != 4 {
		return ""
	}
	ihl := int(l3[0]&0x0f) * 4
	if l3[9] != 1 || len(l3) < ihl+4 {
		return ""
	}
	typ := l3[ihl]
	switch typ {
	case 0:
		return "echo-reply"
	case 3:
		if len(l3) > ihl+3 {
			return fmt.Sprintf("unreachable(code=%d)", l3[ihl+3])
		}
		return "unreachable"
	case 8:
		return "echo-request"
	case 11:
		return "time-exceeded"
	default:
		return fmt.Sprintf("type-%d", typ)
	}
}

func parseICMPv6(l3 []byte) string {
	if len(l3) < 40 || (l3[0]>>4) != 6 || l3[6] != 58 {
		return ""
	}
	typ := l3[40]
	switch typ {
	case 128:
		return "echo-request"
	case 129:
		return "echo-reply"
	default:
		return fmt.Sprintf("icmpv6-type-%d", typ)
	}
}

func parseDNSResponse(frame []byte) (rcode string, isResponse bool) {
	qname, rcode, isResp := parseDNS(frame)
	_ = qname
	return rcode, isResp
}

func parseDNS(frame []byte) (qname string, rcode string, isResponse bool) {
	frame = ethernetPayload(frame)
	if len(frame) < 14 {
		return "", "", false
	}
	eth := binary.BigEndian.Uint16(frame[12:14])
	if eth != 0x0800 {
		return "", "", false
	}
	l3 := frame[14:]
	if len(l3) < 20 || l3[9] != 17 {
		return "", "", false
	}
	ihl := int(l3[0]&0x0f) * 4
	off := 14 + ihl + 8
	if len(frame) < off+12 {
		return "", "", false
	}
	dns := frame[off:]
	flags := binary.BigEndian.Uint16(dns[2:4])
	isResponse = flags&0x8000 != 0
	rcode = dnsRcode(flags)
	if !isResponse {
		return parseDNSQuery(dns[12:]), rcode, false
	}
	return "", rcode, true
}

func dnsRcode(flags uint16) string {
	rc := flags & 0x000f
	switch rc {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", rc)
	}
}

func parseTLSSNI(frame []byte) string {
	frame = ethernetPayload(frame)
	if len(frame) < 14 {
		return ""
	}
	eth := binary.BigEndian.Uint16(frame[12:14])
	if eth != 0x0800 {
		return ""
	}
	l3 := frame[14:]
	if len(l3) < 20 || l3[9] != 6 {
		return ""
	}
	ihl := int(l3[0]&0x0f) * 4
	tcpOff := 14 + ihl
	if len(frame) < tcpOff+20 {
		return ""
	}
	dataOff := tcpOff + int((frame[tcpOff+12]>>4)&0x0f)*4
	if len(frame) <= dataOff+5 {
		return ""
	}
	payload := frame[dataOff:]
	// TLS record: 0x16 handshake, version, length
	if len(payload) < 5 || payload[0] != 0x16 {
		return ""
	}
	// handshake message client hello
	if len(payload) < 9 || payload[5] != 0x01 {
		return ""
	}
	// skip: handshake type(1)+len(3)+client version(2)+random(32)+sess id len(1)+...
	pos := 5 + 4 + 2 + 32
	if len(payload) <= pos {
		return ""
	}
	sessLen := int(payload[pos])
	pos++
	if len(payload) < pos+sessLen+2 {
		return ""
	}
	pos += sessLen
	cipherLen := int(binary.BigEndian.Uint16(payload[pos : pos+2]))
	pos += 2 + cipherLen
	if len(payload) < pos+1 {
		return ""
	}
	compLen := int(payload[pos])
	pos++
	if len(payload) < pos+compLen+2 {
		return ""
	}
	pos += compLen
	extLen := int(binary.BigEndian.Uint16(payload[pos : pos+2]))
	pos += 2
	end := pos + extLen
	for pos+4 <= end && pos+4 <= len(payload) {
		typ := binary.BigEndian.Uint16(payload[pos : pos+2])
		ln := int(binary.BigEndian.Uint16(payload[pos+2 : pos+4]))
		pos += 4
		if pos+ln > len(payload) {
			break
		}
		if typ == 0 {
			return parseSNIName(payload[pos : pos+ln])
		}
		pos += ln
	}
	return ""
}

func parseSNIName(b []byte) string {
	if len(b) < 3 {
		return ""
	}
	// name type host_name(0), length, labels
	if b[0] != 0 {
		return ""
	}
	return parseDNSQuery(b[2:])
}

func sortedBuckets(m map[int]*TimeBucket) []TimeBucket {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]TimeBucket, 0, len(keys))
	for _, k := range keys {
		out = append(out, *m[k])
	}
	return out
}

func topSNI(m map[string]int, n int) []SNIStat {
	out := make([]SNIStat, 0, len(m))
	for h, c := range m {
		out = append(out, SNIStat{Host: h, Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
