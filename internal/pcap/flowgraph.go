package pcap

import (
	"fmt"
	"strings"
)

// FlowEdge is a directed workload flow for diagram rendering.
type FlowEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Proto     string `json:"proto,omitempty"`
	SrcPort   uint16 `json:"src_port,omitempty"`
	DstPort   uint16 `json:"dst_port,omitempty"`
	Count     int    `json:"count"`
}

// FlowGraph is nodes + edges for the AI popup diagram.
type FlowGraph struct {
	Nodes []string   `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
	Mermaid string   `json:"mermaid"`
}

// BuildFlowGraph aggregates capture events into a simple topology.
func BuildFlowGraph(events []FlowEvent, tracked []TrackedPod) FlowGraph {
	type key struct{ from, to string }
	counts := map[key]*FlowEdge{}
	nodeSet := map[string]struct{}{}

	for _, ev := range events {
		fs := summarizeFrame(ethernetPayload(ev.Frame))
		fromN, toN := resolveFlowEndpoints(ev, tracked)
		from, to := fromN.ID, toN.ID
		if from == "" || to == "" {
			continue
		}
		nodeSet[from] = struct{}{}
		nodeSet[to] = struct{}{}
		k := key{from: from, to: to}
		e, ok := counts[k]
		if !ok {
			e = &FlowEdge{From: from, To: to, Proto: fs.Proto, SrcPort: fs.SrcPort, DstPort: fs.DstPort}
			counts[k] = e
		}
		e.Count++
		if fs.Proto != "" {
			e.Proto = fs.Proto
		}
	}

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	edges := make([]FlowEdge, 0, len(counts))
	for _, e := range counts {
		edges = append(edges, *e)
	}
	return FlowGraph{
		Nodes: nodes,
		Edges: edges,
		Mermaid: mermaidFromEdges(edges),
	}
}

func endpointLabel(meta map[string]interface{}, src bool, fallback string) string {
	if meta == nil {
		return fallback
	}
	prefix := "Dst"
	if src {
		prefix = "Src"
	}
	ns, _ := meta[prefix+"K8S_Namespace"].(string)
	name, _ := meta[prefix+"K8S_Name"].(string)
	owner, _ := meta[prefix+"K8S_OwnerName"].(string)
	kind, _ := meta[prefix+"K8S_OwnerType"].(string)
	if ns != "" && name != "" {
		return ns + "/" + name
	}
	if ns != "" && kind != "" && owner != "" {
		return ns + "/" + kind + "/" + owner
	}
	return fallback
}

func mermaidFromEdges(edges []FlowEdge) string {
	if len(edges) == 0 {
		return "flowchart LR\n  idle[No K8s-enriched flows yet]\n"
	}
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	seen := map[string]struct{}{}
	id := func(s string) string {
		out := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				return r
			default:
				return '_'
			}
		}, s)
		if out == "" {
			out = "node"
		}
		return out
	}
	for _, e := range edges {
		fid, tid := id(e.From), id(e.To)
		if _, ok := seen[fid]; !ok {
			b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", fid, e.From))
			seen[fid] = struct{}{}
		}
		if _, ok := seen[tid]; !ok {
			b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", tid, e.To))
			seen[tid] = struct{}{}
		}
		lbl := e.Proto
		if e.DstPort > 0 {
			if lbl != "" {
				lbl += fmt.Sprintf(":%d", e.DstPort)
			} else {
				lbl = fmt.Sprintf("%d", e.DstPort)
			}
		}
		if lbl == "" {
			lbl = fmt.Sprintf("%d pkts", e.Count)
		} else {
			lbl = fmt.Sprintf("%s (%d)", lbl, e.Count)
		}
		b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", fid, lbl, tid))
	}
	return b.String()
}
