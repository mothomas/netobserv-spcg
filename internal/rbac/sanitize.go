package rbac

import (
	"strings"

	"github.com/netobserv/spcg/internal/trace"
	"github.com/netobserv/spcg/internal/trace/engine"
)

// SanitizeTopologyResult strips infra-sensitive nodes/edges for namespace-scoped tenants.
func SanitizeTopologyResult(topo *engine.TopologyResult) *engine.TopologyResult {
	if topo == nil {
		return nil
	}
	out := *topo
	out.Infrastructure = engine.InfrastructurePlaneResult{}
	out.Physical = engine.PhysicalPlaneResult{}
	out.Nodes = filterNodes(topo.Nodes)
	out.Edges = filterEdges(topo.Nodes, topo.Edges)
	out.Graph = sanitizeTraceGraph(topo.Graph)
	out.Layers = collapseLayers(topo.Layers)
	return &out
}

func filterNodes(nodes []engine.TopologyNode) []engine.TopologyNode {
	out := make([]engine.TopologyNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Sensitive || n.Layer != engine.LayerLogical {
			continue
		}
		n.Label = abstractLabel(n)
		n.Detail = abstractDetail(n)
		out = append(out, n)
	}
	return out
}

func filterEdges(nodes []engine.TopologyNode, edges []engine.TopologyEdge) []engine.TopologyEdge {
	allowed := map[string]struct{}{}
	for _, n := range filterNodes(nodes) {
		allowed[n.ID] = struct{}{}
	}
	out := make([]engine.TopologyEdge, 0, len(edges))
	for _, e := range edges {
		if e.Layer != engine.LayerLogical {
			continue
		}
		if _, ok := allowed[e.From]; !ok {
			continue
		}
		if _, ok := allowed[e.To]; !ok {
			continue
		}
		e.OpenFlowCookie = ""
		e.ACLMetadata = ""
		out = append(out, e)
	}
	return out
}

func sanitizeTraceGraph(g trace.TraceGraph) trace.TraceGraph {
	out := g
	out.Nodes = make([]trace.TraceNode, 0, len(g.Nodes))
	out.Edges = make([]trace.TraceEdge, 0, len(g.Edges))
	keep := map[string]struct{}{}
	for _, n := range g.Nodes {
		if n.Layer == "physical" || isInfraKind(n.Kind) {
			continue
		}
		if isInfraKind(n.Kind) {
			n.Label = "Infrastructure"
			n.Detail = ""
		}
		if n.Kind == "node" || n.Kind == "bond" || n.Kind == "bgp-peer" {
			continue
		}
		out.Nodes = append(out.Nodes, n)
		keep[n.ID] = struct{}{}
	}
	for _, e := range g.Edges {
		if _, ok := keep[e.From]; !ok {
			continue
		}
		if _, ok := keep[e.To]; !ok {
			continue
		}
		out.Edges = append(out.Edges, e)
	}
	return out
}

func isInfraKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "node", "bond", "host-veth", "vrf", "bgp-peer", "metallb-pool", "metallb-advertisement", "ovs-bridge", "logical-switch", "logical-router":
		return true
	default:
		return false
	}
}

func abstractLabel(n engine.TopologyNode) string {
	switch n.Neo4jLabel {
	case "Service":
		return n.Label
	case "Pod":
		return n.Label
	case "NetworkAttachmentDefinition":
		return "Secondary network"
	case "EgressIP":
		return "Egress gateway"
	case "LoadBalancer", "BGP_Peer":
		return "External entry"
	default:
		return n.Label
	}
}

func abstractDetail(n engine.TopologyNode) string {
	if n.Namespace != "" {
		return n.Namespace
	}
	return ""
}

func collapseLayers(layers engine.VisualizationLayers) engine.VisualizationLayers {
	return engine.VisualizationLayers{
		Logical: engine.LayerRegion{
			Label: "Logical path", Anchor: "top-left",
			X: layers.Logical.X, Y: layers.Logical.Y,
			Width: layers.Logical.Width, Height: layers.Logical.Height,
		},
	}
}

// ApplyTraceSanitization mutates topology in place when tenant RBAC requires it.
func ApplyTraceSanitization(topo *engine.TopologyResult, tc TraceContext) *engine.TopologyResult {
	if topo == nil || !tc.SanitizeInfra {
		return topo
	}
	return SanitizeTopologyResult(topo)
}
