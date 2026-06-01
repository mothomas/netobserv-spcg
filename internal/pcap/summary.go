package pcap

import (
	"sort"
	"strings"
	"time"
)

// CaptureSummary aggregates live capture for the stats bar.
type CaptureSummary struct {
	EventCount         int                `json:"event_count"`
	TotalPackets       uint64             `json:"total_packets"`
	TotalBytes         uint64             `json:"total_bytes"`
	FlowEdges          int                `json:"flow_edges"`
	UniqueNodes        int                `json:"unique_nodes"`
	ExternalPeers      int                `json:"external_peers"`
	TrackedPods        int                `json:"tracked_pods"`
	CaptureDurationSec float64            `json:"capture_duration_sec"`
	EventsPerSec       float64            `json:"events_per_sec"`
	AvgPacketBytes     float64            `json:"avg_packet_bytes"`
	AvgSrttMs          float64            `json:"avg_srtt_ms"`
	Protocols          map[string]int     `json:"protocols"`
	Health             map[string]int     `json:"health"`
	TCPFlags           map[string]int     `json:"tcp_flags"`
	TopPorts           []PortStat         `json:"top_ports"`
	TopDNS             []DNSStat          `json:"top_dns"`
	TopTalkers         []TalkerStat       `json:"top_talkers"`
	DropEdges          int                `json:"drop_edges"`
	DnsQueries         int                `json:"dns_queries"`
}

type TalkerStat struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind,omitempty"`
	Packets uint64 `json:"packets"`
	Bytes   uint64 `json:"bytes"`
}

type PortStat struct {
	Proto   string `json:"proto"`
	Port    uint16 `json:"port"`
	Count   int    `json:"count"`
	Bytes   uint64 `json:"bytes"`
}

type DNSStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// BuildCaptureSummary derives UI stats from events and filtered topology.
func BuildCaptureSummary(events []FlowEvent, tracked []TrackedPod) CaptureSummary {
	topo := BuildFlowTopology(events, tracked)
	if ids := trackedNodeIDs(tracked); len(ids) > 0 {
		topo = FilterTopologyToSelection(topo, ids)
	}

	sum := CaptureSummary{
		EventCount:  len(events),
		TrackedPods: len(tracked),
		Protocols:   map[string]int{},
		Health:      map[string]int{},
		TCPFlags:    map[string]int{},
		FlowEdges:   len(topo.Edges),
		UniqueNodes: len(topo.Nodes),
	}

	for _, ev := range events {
		sum.TotalPackets++
		if len(ev.Frame) > 0 {
			sum.TotalBytes += uint64(len(ethernetPayload(ev.Frame)))
		} else {
			sum.TotalBytes++
		}
	}

	var first, last time.Time
	portCounts := map[string]*PortStat{}
	dnsCounts := map[string]int{}
	talkerBytes := map[string]uint64{}
	talkerPkts := map[string]uint64{}
	talkerLabel := map[string]string{}
	talkerKind := map[string]string{}
	var srttSum int64
	var srttN int

	for _, n := range topo.Nodes {
		talkerLabel[n.ID] = n.Label
		talkerKind[n.ID] = n.Kind
		if n.Kind == "External" {
			sum.ExternalPeers++
		}
	}

	for _, e := range topo.Edges {
		pkts := e.Packets
		if pkts == 0 {
			pkts = uint64(e.Count)
		}
		p := e.Proto
		if p == "" {
			p = "OTHER"
		}
		sum.Protocols[p] += e.Count
		h := e.Health
		if h == "" {
			h = "healthy"
		}
		sum.Health[h] += e.Count
		if h == "dropped" || e.DropCause != "" {
			sum.DropEdges++
		}
		if e.SrttNs > 0 {
			srttSum += e.SrttNs
			srttN++
		}
		for _, f := range e.TcpFlags {
			sum.TCPFlags[f]++
		}
		portKey := ""
		if e.DstPort > 0 {
			portKey = e.Proto + "/dst:" + itoaU16(e.DstPort)
		}
		if portKey != "" {
			ps, ok := portCounts[portKey]
			if !ok {
				ps = &PortStat{Proto: e.Proto, Port: e.DstPort}
				portCounts[portKey] = ps
			}
			ps.Count += e.Count
			ps.Bytes += e.Bytes
		}
		for _, id := range []string{e.From, e.To} {
			talkerBytes[id] += e.Bytes
			talkerPkts[id] += pkts
		}
	}

	for i, ev := range events {
		if i == 0 || ev.At.Before(first) {
			first = ev.At
		}
		if ev.At.After(last) {
			last = ev.At
		}
		fs := summarizeFrame(ethernetPayload(ev.Frame))
		fs = mergeFrameMeta(fs, ev.FlowMeta)
		if fs.DNSQuery != "" {
			sum.DnsQueries++
			dnsCounts[fs.DNSQuery]++
		}
		if fs.Proto != "" {
			sum.Protocols[fs.Proto]++
		}
		for _, f := range fs.TCPFlags {
			sum.TCPFlags[f]++
		}
		if fs.DstPort > 0 {
			key := fs.Proto + "/dst:" + itoaU16(fs.DstPort)
			ps, ok := portCounts[key]
			if !ok {
				ps = &PortStat{Proto: fs.Proto, Port: fs.DstPort}
				portCounts[key] = ps
			}
			ps.Count++
			if len(ev.Frame) > 0 {
				ps.Bytes += uint64(len(ev.Frame))
			}
		}
	}

	if !first.IsZero() && !last.IsZero() && last.After(first) {
		sec := last.Sub(first).Seconds()
		sum.CaptureDurationSec = sec
		if sec > 0 {
			sum.EventsPerSec = float64(len(events)) / sec
		}
	}
	if sum.TotalPackets > 0 {
		sum.AvgPacketBytes = float64(sum.TotalBytes) / float64(sum.TotalPackets)
	}
	if srttN > 0 {
		sum.AvgSrttMs = float64(srttSum) / float64(srttN) / 1e6
	}

	sum.TopPorts = topPorts(portCounts, 8)
	sum.TopDNS = topDNS(dnsCounts, 6)
	sum.TopTalkers = topTalkers(talkerBytes, talkerPkts, talkerLabel, talkerKind, 8)

	return sum
}

func itoaU16(p uint16) string {
	if p == 0 {
		return "0"
	}
	var b [5]byte
	n := len(b)
	v := uint(p)
	for v > 0 {
		n--
		b[n] = byte('0' + v%10)
		v /= 10
	}
	return string(b[n:])
}

func topPorts(m map[string]*PortStat, n int) []PortStat {
	out := make([]PortStat, 0, len(m))
	for _, p := range m {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func topDNS(m map[string]int, n int) []DNSStat {
	out := make([]DNSStat, 0, len(m))
	for name, c := range m {
		out = append(out, DNSStat{Name: name, Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func topTalkers(bytes, pkts map[string]uint64, labels, kinds map[string]string, n int) []TalkerStat {
	out := make([]TalkerStat, 0, len(bytes))
	for id, b := range bytes {
		out = append(out, TalkerStat{
			ID: id, Label: labels[id], Kind: kinds[id], Packets: pkts[id], Bytes: b,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Packets > out[j].Packets
		}
		return out[i].Bytes > out[j].Bytes
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// FormatProtoMix renders protocol counts for UI chips.
func FormatProtoMix(m map[string]int) string {
	if len(m) == 0 {
		return ""
	}
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(m))
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].v > items[j].v })
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, it.k+" "+itoaInt(it.v))
	}
	return strings.Join(parts, " · ")
}

func itoaInt(v int) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	n := len(b)
	x := v
	if x < 0 {
		x = -x
	}
	for x > 0 {
		n--
		b[n] = byte('0' + x%10)
		x /= 10
	}
	return string(b[n:])
}
