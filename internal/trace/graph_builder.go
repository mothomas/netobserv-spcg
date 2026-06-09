package trace

import (
	"fmt"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

const (
	rankSource = 0
	rankDest   = 5
)

type graphBuilder struct {
	sourcePods []spcgk8s.PodDetail
	destPods   []spcgk8s.PodDetail
	destIP     *ipEndpointNode
	scope      map[string]struct{}
	anchor     spcgk8s.PodDetail
	sourceIDs  map[string]struct{}
	destIDs    map[string]struct{}
	nodes      []TraceNode
	edges      []TraceEdge
	paths      []PathSummary
	nodeSet    map[string]struct{}
	edgeSet    map[string]struct{}
}

func newGraphBuilder(sourcePods, destPods []spcgk8s.PodDetail, destIP *ipEndpointNode, scope map[string]struct{}) *graphBuilder {
	anchor := spcgk8s.PodDetail{}
	if len(sourcePods) > 0 {
		anchor = sourcePods[0]
	}
	b := &graphBuilder{
		sourcePods: sourcePods,
		destPods:   destPods,
		destIP:     destIP,
		scope:      scope,
		anchor:     anchor,
		sourceIDs:  map[string]struct{}{},
		destIDs:    map[string]struct{}{},
		nodeSet:    map[string]struct{}{},
		edgeSet:    map[string]struct{}{},
	}
	for _, p := range sourcePods {
		b.sourceIDs[podNodeID(p)] = struct{}{}
	}
	for _, p := range destPods {
		b.destIDs[podNodeID(p)] = struct{}{}
	}
	if destIP != nil {
		b.destIDs[destIP.ID] = struct{}{}
	}
	return b
}

func podNodeID(p spcgk8s.PodDetail) string {
	return nodeID("pod", p.Namespace, p.Name)
}

func (b *graphBuilder) hasNode(id string) bool {
	_, ok := b.nodeSet[id]
	return ok
}

func (b *graphBuilder) anchorPodID() string {
	if b.anchor.Name == "" {
		return ""
	}
	return podNodeID(b.anchor)
}

func (b *graphBuilder) anchorNodeID() string {
	if b.anchor.NodeName == "" {
		return ""
	}
	return nodeID("node", "", b.anchor.NodeName)
}

func (b *graphBuilder) seedEndpoints() {
	for _, p := range b.sourcePods {
		id := podNodeID(p)
		b.addNode(id, p.Name, "pod", p.Namespace, true, false, rankSource, p.PodIP)
		if p.NodeName != "" {
			nid := nodeID("node", "", p.NodeName)
			b.addNode(nid, p.NodeName, "node", "", false, false, rankForKind("node"), "scheduled node")
			b.addEdge(id, nid, "scheduled", false, "")
		}
	}
	for _, p := range b.destPods {
		id := podNodeID(p)
		b.addNode(id, p.Name, "pod", p.Namespace, true, false, rankDest, p.PodIP)
	}
	if b.destIP != nil {
		b.addNode(b.destIP.ID, b.destIP.Label, b.destIP.Kind, "", true, false, rankDest, b.destIP.Detail)
	}
}

func (b *graphBuilder) addNode(id, label, kind, ns string, tracked, focused bool, rank int, detail string) {
	if _, ok := b.nodeSet[id]; ok {
		return
	}
	b.nodeSet[id] = struct{}{}
	if rank < 0 {
		rank = rankForKind(kind)
	}
	b.nodes = append(b.nodes, TraceNode{
		ID:        id,
		Label:     label,
		Kind:      kind,
		Namespace: ns,
		Rank:      rank,
		Tracked:   tracked,
		Focused:   focused,
		Status:    string(HopPending),
		Detail:    detail,
	})
}

func (b *graphBuilder) addEdge(from, to, edgeType string, primary bool, label string, drop ...bool) {
	if from == "" || to == "" || from == to {
		return
	}
	key := from + "->" + to + ":" + edgeType
	if _, ok := b.edgeSet[key]; ok {
		return
	}
	b.edgeSet[key] = struct{}{}
	isDrop := len(drop) > 0 && drop[0]
	b.edges = append(b.edges, TraceEdge{
		ID:       fmt.Sprintf("e-%d", len(b.edges)+1),
		From:     from,
		To:       to,
		EdgeType: edgeType,
		Primary:  primary,
		Drop:     isDrop,
		Label:    label,
	})
}

func (b *graphBuilder) addPath(direction, resource, ns, kind, status, detail string) {
	b.paths = append(b.paths, PathSummary{
		Direction: direction,
		Resource:  resource,
		Namespace: ns,
		Kind:      kind,
		Status:    status,
		Detail:    detail,
	})
}

func (b *graphBuilder) finish(traceID string) TraceGraph {
	markFocusPath(b)
	anchorID := b.anchorPodID()
	pathOpts := buildPathOptions(b.nodes, b.edges, b.paths, anchorID)
	applyPathRefs(b.nodes, b.edges, pathOpts, anchorID)

	var g TraceGraph
	var lanes []TraceLane
	if len(pathOpts) > 0 {
		g, lanes = applyPathLayout(b.nodes, pathOpts, anchorID)
	} else {
		g, lanes = applyLayout(b.nodes)
	}
	g.Edges = b.edges
	g.Paths = b.paths
	g.PathOptions = pathOpts
	g.AnchorNodeID = anchorID
	g.TraceID = traceID
	g.Lanes = lanes
	ns := make([]string, 0, len(b.scope))
	for n := range b.scope {
		ns = append(ns, n)
	}
	g.Namespaces = ns
	return g
}

func nodeID(kind, ns, name string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	ns = strings.TrimSpace(ns)
	name = strings.TrimSpace(name)
	if ns == "" {
		return kind + "/" + name
	}
	return kind + "/" + ns + "/" + name
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
