package trace

import "strings"

// Path tracks separate ingress discovery from egress discovery in the path map.
const (
	TrackIngress = "ingress"
	TrackEgress  = "egress"
	TrackAnchor  = "anchor"
	TrackShared  = "shared"
	TrackContext = "context"
)

func inferPathTrack(n TraceNode, edges []TraceEdge, paths []PathSummary) string {
	if n.Kind == "networkpolicy" {
		return TrackContext
	}
	if n.Tracked && n.Kind == "pod" {
		return TrackAnchor
	}
	if n.Tracked && (n.Kind == "external" || n.Kind == "external-client") {
		return TrackEgress
	}

	for _, p := range paths {
		if p.Status != "discovered" {
			continue
		}
		if !pathMatchesNode(p, n) {
			continue
		}
		switch p.Direction {
		case "ingress":
			return TrackIngress
		case "egress":
			return TrackEgress
		case "host":
			if n.Focused {
				return TrackShared
			}
			return TrackContext
		}
	}

	switch n.Kind {
	case "external-client", "loadbalancer-external", "metallb-pool", "metallb-advertisement", "route":
		return TrackIngress
	case "bgp-peer":
		return TrackIngress
	case "egressip", "egressservice", "egress-router":
		return TrackEgress
	case "external":
		return TrackEgress
	case "node", "bond", "ovn-logical-port", "host-veth", "nad", "vrf":
		if n.Focused {
			return TrackShared
		}
		return TrackContext
	case "service-clusterip", "service-loadbalancer", "service-nodeport", "service":
		ing, egr := edgeDirectionFlags(n.ID, edges)
		if ing && !egr {
			return TrackIngress
		}
		if egr && !ing {
			return TrackEgress
		}
		if n.Focused {
			return TrackIngress
		}
		return TrackContext
	}

	ing, egr := edgeDirectionFlags(n.ID, edges)
	if ing && !egr {
		return TrackIngress
	}
	if egr && !ing {
		return TrackEgress
	}
	if n.Focused {
		return TrackEgress
	}
	if !n.Tracked {
		return TrackContext
	}
	return TrackShared
}

func pathMatchesNode(p PathSummary, n TraceNode) bool {
	if p.Resource != "" && strings.EqualFold(p.Resource, n.Label) {
		return true
	}
	if p.Kind != "" && strings.EqualFold(p.Kind, n.Kind) {
		if p.Namespace == "" || p.Namespace == n.Namespace {
			return true
		}
	}
	return false
}

func edgeDirectionFlags(nodeID string, edges []TraceEdge) (ingress, egress bool) {
	for _, e := range edges {
		if e.From != nodeID && e.To != nodeID {
			continue
		}
		switch e.EdgeType {
		case "ingress":
			ingress = true
		case "egress", "egressservice":
			egress = true
		}
	}
	return ingress, egress
}
