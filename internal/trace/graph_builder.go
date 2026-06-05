package trace

import (
	"fmt"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

type graphBuilder struct {
	target   spcgk8s.PodDetail
	scope    map[string]struct{}
	podID    string
	nodeID   string
	nodes    []TraceNode
	edges    []TraceEdge
	paths    []PathSummary
	nodeSet  map[string]struct{}
	edgeSet  map[string]struct{}
}

func newGraphBuilder(target spcgk8s.PodDetail, scope map[string]struct{}) *graphBuilder {
	podID := nodeID("pod", target.Namespace, target.Name)
	return &graphBuilder{
		target:  target,
		scope:   scope,
		podID:   podID,
		nodeSet: map[string]struct{}{},
		edgeSet: map[string]struct{}{},
	}
}

func (b *graphBuilder) hasNode(id string) bool {
	_, ok := b.nodeSet[id]
	return ok
}

func (b *graphBuilder) addNode(id, label, kind, ns string, tracked bool, detail string) {
	if _, ok := b.nodeSet[id]; ok {
		return
	}
	b.nodeSet[id] = struct{}{}
	if id == nodeID("node", "", b.target.NodeName) {
		b.nodeID = id
	}
	b.nodes = append(b.nodes, TraceNode{
		ID:        id,
		Label:     label,
		Kind:      kind,
		Namespace: ns,
		Rank:      rankForKind(kind),
		Tracked:   tracked,
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
	g, lanes := applyLayout(b.nodes)
	g.Edges = b.edges
	g.Paths = b.paths
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
