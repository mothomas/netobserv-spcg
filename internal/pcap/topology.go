package pcap

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// TopologyNode is a workload vertex for the observability map.
type TopologyNode struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Kind       string `json:"kind,omitempty"` // Pod, Service, External
	Namespace  string `json:"namespace"`
	Pod        string `json:"pod,omitempty"`
	OwnerKind  string `json:"owner_kind,omitempty"`
	OwnerName  string `json:"owner_name,omitempty"`
	HostName   string `json:"host_name,omitempty"`
	HostIP     string `json:"host_ip,omitempty"`
}

// TopologyEdge is a directed flow between workloads with health hints.
type TopologyEdge struct {
	ID            string   `json:"id"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Health        string   `json:"health"` // healthy, degraded, dropped
	Proto         string   `json:"proto,omitempty"`
	SrcPort       uint16   `json:"src_port,omitempty"`
	DstPort       uint16   `json:"dst_port,omitempty"`
	Count         int      `json:"count"`
	Bytes         uint64   `json:"bytes"`
	Packets       uint64   `json:"packets"`
	SrttNs        int64    `json:"srtt_ns,omitempty"`
	MaxSrttNs     int64    `json:"max_srtt_ns,omitempty"`
	DropCause     string   `json:"drop_cause,omitempty"`
	DropDiagnosis string   `json:"drop_diagnosis,omitempty"`
	TcpFlags      []string `json:"tcp_flags,omitempty"`
	TcpState      string   `json:"tcp_state,omitempty"`
}

// SequenceStep is one packet marker on the ladder diagram.
type SequenceStep struct {
	RelUs  int64    `json:"rel_us"`
	Lane   string   `json:"lane"`
	Label  string   `json:"label"`
	Flags  []string `json:"flags,omitempty"`
}

// EdgeDetail holds deep metrics and sequence for a selected edge.
type EdgeDetail struct {
	EdgeID        string         `json:"edge_id"`
	SrttNs        int64          `json:"srtt_ns,omitempty"`
	Bytes         uint64         `json:"bytes"`
	Packets       uint64         `json:"packets"`
	TcpFlags      []string       `json:"tcp_flags,omitempty"`
	TcpState      string         `json:"tcp_state,omitempty"`
	DropCause     string         `json:"drop_cause,omitempty"`
	DropDiagnosis string         `json:"drop_diagnosis,omitempty"`
	Sequence      []SequenceStep `json:"sequence,omitempty"`
}

// FlowTopology is the rich graph returned to the UI.
type FlowTopology struct {
	Nodes      []TopologyNode        `json:"nodes"`
	Edges      []TopologyEdge        `json:"edges"`
	Namespaces []string              `json:"namespaces"`
	EdgeDetail map[string]EdgeDetail `json:"edge_details,omitempty"`
}

// BuildFlowTopology aggregates capture events into a workload graph.
func BuildFlowTopology(events []FlowEvent, tracked []TrackedPod) FlowTopology {
	type edgeKey struct{ from, to string }
	type edgeAcc struct {
		edge   TopologyEdge
		flags  map[string]struct{}
		steps  []SequenceStep
		first  time.Time
		srttSum int64
		srttN   int
	}

	nodeMap := map[string]TopologyNode{}
	edges := map[edgeKey]*edgeAcc{}

	for _, ev := range events {
		fs := summarizeFrame(ethernetPayload(ev.Frame))
		fs = mergeFrameMeta(fs, ev.FlowMeta)
		fromN, toN := resolveFlowEndpoints(ev, tracked)
		if fromN.ID == "" || toN.ID == "" {
			continue
		}
		nodeMap[fromN.ID] = fromN
		nodeMap[toN.ID] = toN

		k := edgeKey{from: fromN.ID, to: toN.ID}
		acc, ok := edges[k]
		if !ok {
			acc = &edgeAcc{
				edge: TopologyEdge{
					ID:    edgeID(fromN.ID, toN.ID),
					From:  fromN.ID,
					To:    toN.ID,
					Proto: fs.Proto,
				},
				flags: map[string]struct{}{},
			}
			edges[k] = acc
		}
		e := &acc.edge
		e.Count++
		if fs.Proto != "" {
			e.Proto = fs.Proto
		}
		if fs.SrcPort > 0 {
			e.SrcPort = fs.SrcPort
		}
		if fs.DstPort > 0 {
			e.DstPort = fs.DstPort
		}

		bytes, packets := flowCounters(ev.FlowMeta, ev.Frame)
		e.Bytes += bytes
		e.Packets += packets

		srtt := flowInt64(ev.FlowMeta, "TimeFlowRttNs")
		if srtt > 0 {
			acc.srttSum += srtt
			acc.srttN++
			if srtt > e.MaxSrttNs {
				e.MaxSrttNs = srtt
			}
		}

		drop := flowString(ev.FlowMeta, "PktDropLatestDropCause")
		if drop == "" {
			drop = flowString(ev.FlowMeta, "PktDropLatestState")
		}
		if drop != "" {
			e.DropCause = drop
			e.DropDiagnosis = DiagnoseDrop(drop)
		}
		if flowUint(ev.FlowMeta, "PktDropPackets") > 0 || flowUint(ev.FlowMeta, "PktDropBytes") > 0 {
			if e.DropCause == "" {
				e.DropCause = "SKB_DROP"
			}
			e.DropDiagnosis = DiagnoseDrop(e.DropCause)
		}

		tcpFlags := tcpFlagsFromMeta(ev.FlowMeta)
		if len(tcpFlags) == 0 {
			tcpFlags = fs.TCPFlags
		}
		if len(tcpFlags) == 0 {
			tcpFlags = parseTCPFlags(ethernetPayload(ev.Frame))
		}
		for _, f := range tcpFlags {
			acc.flags[f] = struct{}{}
		}
		state := flowString(ev.FlowMeta, "PktDropLatestState")
		if state != "" {
			e.TcpState = state
		}

		if len(acc.steps) < 48 {
			if acc.first.IsZero() {
				acc.first = ev.At
			}
			rel := ev.At.Sub(acc.first).Microseconds()
			lane := sequenceLane(fromN, toN)
			lbl := sequenceLabel(fs, tcpFlags)
			acc.steps = append(acc.steps, SequenceStep{RelUs: rel, Lane: lane, Label: lbl, Flags: tcpFlags})
		}
	}

	nsSet := map[string]struct{}{}
	nodes := make([]TopologyNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
		if n.Namespace != "" {
			nsSet[n.Namespace] = struct{}{}
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	outEdges := make([]TopologyEdge, 0, len(edges))
	details := map[string]EdgeDetail{}
	for _, acc := range edges {
		e := acc.edge
		if acc.srttN > 0 {
			e.SrttNs = acc.srttSum / int64(acc.srttN)
		}
		e.Health = edgeHealth(e)
		for f := range acc.flags {
			e.TcpFlags = append(e.TcpFlags, f)
		}
		sort.Strings(e.TcpFlags)
		outEdges = append(outEdges, e)
		details[e.ID] = EdgeDetail{
			EdgeID:        e.ID,
			SrttNs:        e.SrttNs,
			Bytes:         e.Bytes,
			Packets:       e.Packets,
			TcpFlags:      e.TcpFlags,
			TcpState:      e.TcpState,
			DropCause:     e.DropCause,
			DropDiagnosis: e.DropDiagnosis,
			Sequence:      acc.steps,
		}
	}
	sort.Slice(outEdges, func(i, j int) bool { return outEdges[i].ID < outEdges[j].ID })

	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	return FlowTopology{
		Nodes:      nodes,
		Edges:      outEdges,
		Namespaces: namespaces,
		EdgeDetail: details,
	}
}

func edgeID(from, to string) string {
	return from + "->" + to
}

func mergeFrameMeta(fs FrameSummary, meta map[string]interface{}) FrameSummary {
	if fs.Proto == "" {
		fs.Proto = flowString(meta, "Proto")
	}
	if fs.SrcPort == 0 {
		fs.SrcPort = uint16(flowInt64(meta, "SrcPort"))
	}
	if fs.DstPort == 0 {
		fs.DstPort = uint16(flowInt64(meta, "DstPort"))
	}
	return fs
}

func nodeFromMeta(meta map[string]interface{}, src bool, fallback string) TopologyNode {
	prefix := "Dst"
	if src {
		prefix = "Src"
	}
	ns := flowString(meta, prefix+"K8S_Namespace")
	pod := flowString(meta, prefix+"K8S_Name")
	kind := flowString(meta, prefix+"K8S_OwnerType")
	owner := flowString(meta, prefix+"K8S_OwnerName")
	host := flowString(meta, prefix+"K8S_HostName")
	hostIP := flowString(meta, prefix+"K8S_HostIP")

	label := fallback
	id := fallback
	if ns != "" && pod != "" {
		label = pod
		id = ns + "/" + pod
	} else if ns != "" && kind != "" && owner != "" {
		label = owner
		id = ns + "/" + kind + "/" + owner
	} else if fallback != "" {
		id = fallback
		label = fallback
	} else {
		return TopologyNode{}
	}
	nodeKind := "External"
	if pod != "" {
		nodeKind = "Pod"
	} else if kind != "" {
		nodeKind = kind
	} else if strings.Contains(label, "dns") || strings.HasPrefix(label, "kube-") {
		nodeKind = "Service"
	}
	return TopologyNode{
		ID: id, Label: label, Kind: nodeKind, Namespace: ns, Pod: pod,
		OwnerKind: kind, OwnerName: owner, HostName: host, HostIP: hostIP,
	}
}

// FilterTopologyToSelection keeps only tracked pods and their direct flow neighbors.
func FilterTopologyToSelection(full FlowTopology, tracked map[string]struct{}) FlowTopology {
	if len(tracked) == 0 {
		return full
	}
	touches := func(id string) bool {
		if _, ok := tracked[id]; ok {
			return true
		}
		return false
	}
	edgeKeep := make([]TopologyEdge, 0, len(full.Edges))
	nodeKeep := map[string]struct{}{}
	detailKeep := map[string]EdgeDetail{}
	for _, e := range full.Edges {
		if touches(e.From) || touches(e.To) {
			edgeKeep = append(edgeKeep, e)
			nodeKeep[e.From] = struct{}{}
			nodeKeep[e.To] = struct{}{}
			if d, ok := full.EdgeDetail[e.ID]; ok {
				detailKeep[e.ID] = d
			}
		}
	}
	nodes := make([]TopologyNode, 0, len(nodeKeep))
	nsSet := map[string]struct{}{}
	for _, n := range full.Nodes {
		if _, ok := nodeKeep[n.ID]; ok {
			nodes = append(nodes, n)
			if n.Namespace != "" {
				nsSet[n.Namespace] = struct{}{}
			}
		}
	}
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)
	return FlowTopology{
		Nodes: nodes, Edges: edgeKeep, Namespaces: namespaces, EdgeDetail: detailKeep,
	}
}

func edgeHealth(e TopologyEdge) string {
	if e.DropCause != "" || strings.Contains(strings.ToUpper(e.DropDiagnosis), "DROP") {
		return "dropped"
	}
	if e.MaxSrttNs > 50_000_000 || e.SrttNs > 25_000_000 {
		return "degraded"
	}
	return "healthy"
}

func flowString(m map[string]interface{}, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func flowInt64(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

type jsonNumber float64

func flowUint(m map[string]interface{}, k string) uint64 {
	n := flowInt64(m, k)
	if n < 0 {
		return 0
	}
	return uint64(n)
}

func flowCounters(m map[string]interface{}, frame []byte) (bytes, packets uint64) {
	b := flowInt64(m, "Bytes")
	p := flowInt64(m, "Packets")
	if b > 0 {
		bytes = uint64(b)
	}
	if p > 0 {
		packets = uint64(p)
	}
	if packets == 0 {
		packets = 1
	}
	if bytes == 0 {
		raw := ethernetPayload(frame)
		if len(raw) > 0 {
			bytes = uint64(len(raw))
		}
	}
	return
}

func tcpFlagsFromMeta(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	raw, ok := m["Flags"]
	if !ok || raw == nil {
		return nil
	}
	switch t := raw.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, v := range t {
			out = append(out, fmt.Sprint(v))
		}
		return out
	case string:
		if t != "" {
			return strings.Split(t, ",")
		}
	}
	return nil
}

func sequenceLane(from, to TopologyNode) string {
	if from.Namespace != "" {
		return "src"
	}
	if to.Namespace != "" {
		return "dst"
	}
	return "host"
}

func sequenceLabel(fs FrameSummary, flags []string) string {
	if len(flags) > 0 {
		return strings.Join(flags, "+")
	}
	if fs.Proto != "" {
		return fs.Proto
	}
	return "pkt"
}

// DiagnoseDrop maps netobserv drop causes to plain English.
func DiagnoseDrop(cause string) string {
	c := strings.ToUpper(strings.TrimSpace(cause))
	if c == "" {
		return ""
	}
	rules := []struct {
		sub string
		msg string
	}{
		{"NETFILTER", "Blocked by NetworkPolicy or iptables/netfilter rule"},
		{"NETWORKPOLICY", "Blocked by Kubernetes NetworkPolicy"},
		{"OVS", "Dropped in Open vSwitch bridge or overlay path"},
		{"IPTABLES", "Dropped by host iptables/nftables rule"},
		{"CONNTRACK", "Connection tracking table full or invalid state"},
		{"TTL", "Packet expired (TTL/hop limit exceeded)"},
		{"MTU", "Fragmentation or MTU mismatch on path"},
		{"NO_SOCKET", "No listening socket on destination port"},
		{"TCP_CLOSE", "Connection closed before delivery"},
		{"SKB_DROP", "Kernel dropped packet in network stack"},
	}
	for _, r := range rules {
		if strings.Contains(c, r.sub) {
			return r.msg
		}
	}
	return "Kernel drop: " + cause
}

// MergeTopologyIntoGraph keeps legacy flow_graph fields and adds filtered topology.
func MergeTopologyIntoGraph(events []FlowEvent, tracked []TrackedPod) (FlowGraph, FlowTopology) {
	topo := BuildFlowTopology(events, tracked)
	ids := trackedNodeIDs(tracked)
	if len(ids) > 0 {
		topo = FilterTopologyToSelection(topo, ids)
	}
	legacy := BuildFlowGraph(events, tracked)
	return legacy, topo
}

func trackedNodeIDs(tracked []TrackedPod) map[string]struct{} {
	out := make(map[string]struct{}, len(tracked))
	for _, p := range tracked {
		if p.Namespace != "" && p.Name != "" {
			out[p.Namespace+"/"+p.Name] = struct{}{}
		}
	}
	return out
}

// FormatSrttMs renders RTT for UI.
func FormatSrttMs(ns int64) string {
	if ns <= 0 {
		return "—"
	}
	ms := float64(ns) / 1e6
	if ms < 1 {
		return fmt.Sprintf("%.2f ms", ms)
	}
	return fmt.Sprintf("%.1f ms", math.Round(ms*10)/10)
}
