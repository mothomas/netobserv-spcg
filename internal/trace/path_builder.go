package trace

import (
	"fmt"
	"strings"
)

func buildPathOptions(nodes []TraceNode, edges []TraceEdge, paths []PathSummary, anchorID string) []PathOption {
	if len(paths) == 0 {
		return nil
	}
	adj := directedAdjacency(edges)

	opts := make([]PathOption, 0, len(paths))
	for i, p := range paths {
		id := fmt.Sprintf("path-%d", i)
		dir := PathDirection(p.Direction)
		if dir == "" {
			dir = PathHost
		}
		hops, edgeIDs := inferPathChain(nodes, edges, adj, p, anchorID, dir)
		opts = append(opts, PathOption{
			ID:         id,
			Direction:  dir,
			Mechanism:  pathMechanism(p),
			Label:      pathOptionLabel(p),
			Status:     p.Status,
			Namespace:  p.Namespace,
			HopIDs:     hops,
			EdgeIDs:    edgeIDs,
			Confidence: pathConfidence(hops),
		})
	}
	return opts
}

func pathMechanism(p PathSummary) string {
	k := strings.ToLower(strings.TrimSpace(p.Kind))
	switch k {
	case "route":
		return "openshift-route"
	case "metallb-pool", "loadbalancer-external", "service-loadbalancer":
		return "metallb"
	case "service-nodeport":
		return "nodeport"
	case "service-clusterip", "service":
		return "clusterip"
	case "egressip":
		return "egressip"
	case "egressservice":
		return "egressservice"
	case "bgp-peer":
		return "metallb-bgp"
	case "bond":
		return "default-snat"
	case "nad":
		return "multus"
	case "networkpolicy":
		return "networkpolicy"
	default:
		return k
	}
}

func pathOptionLabel(p PathSummary) string {
	if p.Detail != "" {
		return fmt.Sprintf("%s · %s", p.Resource, truncate(p.Detail, 40))
	}
	return p.Resource
}

func pathConfidence(hops []string) string {
	if len(hops) >= 2 {
		return "inferred"
	}
	return "observed"
}

func directedAdjacency(edges []TraceEdge) map[string][]string {
	out := map[string][]string{}
	for _, e := range edges {
		out[e.From] = append(out[e.From], e.To)
		out[e.To] = append(out[e.To], e.From)
	}
	return out
}

func inferPathChain(nodes []TraceNode, edges []TraceEdge, adj map[string][]string, p PathSummary, anchorID string, dir PathDirection) ([]string, []string) {
	seeds := matchPathNodes(nodes, p)
	if len(seeds) == 0 {
		return nil, nil
	}

	var start, goal string
	switch dir {
	case PathIngress:
		start = pickExternalOrFarthest(seeds, anchorID, adj)
		goal = anchorID
		if goal == "" {
			goal = pickPodTarget(seeds, nodes)
		}
	case PathEgress:
		start = anchorID
		if start == "" {
			start = pickPodTarget(seeds, nodes)
		}
		goal = pickFarthestFrom(seeds, start, adj)
	default:
		start = seeds[0]
		goal = anchorID
	}

	if start == "" || goal == "" || start == goal {
		if len(seeds) == 1 {
			return []string{seeds[0]}, nil
		}
		return seeds, nil
	}

	hops := bfsPath(start, goal, adj)
	if len(hops) < 2 {
		return seeds, nil
	}
	return hops, edgeIDsOnPath(hops, edges)
}

func matchPathNodes(nodes []TraceNode, p PathSummary) []string {
	res := strings.TrimSpace(p.Resource)
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	ns := strings.TrimSpace(p.Namespace)
	out := []string{}
	for _, n := range nodes {
		if ns != "" && n.Namespace != "" && n.Namespace != ns {
			continue
		}
		label := strings.ToLower(n.Label)
		nkind := strings.ToLower(n.Kind)
		if res != "" && (n.Label == res || strings.Contains(label, strings.ToLower(res))) {
			out = append(out, n.ID)
			continue
		}
		if kind != "" && nkind == kind {
			out = append(out, n.ID)
		}
	}
	return uniqueStrings(out)
}

func pickExternalOrFarthest(seeds []string, anchorID string, adj map[string][]string) string {
	for _, id := range seeds {
		if strings.Contains(id, "external") || strings.Contains(id, "client") {
			return id
		}
	}
	if anchorID != "" {
		if far := farthestNode(seeds, anchorID, adj); far != "" {
			return far
		}
	}
	if len(seeds) > 0 {
		return seeds[0]
	}
	return ""
}

func pickFarthestFrom(seeds []string, start string, adj map[string][]string) string {
	if start == "" {
		if len(seeds) > 0 {
			return seeds[len(seeds)-1]
		}
		return ""
	}
	return farthestNode(seeds, start, adj)
}

func pickPodTarget(seeds []string, nodes []TraceNode) string {
	byID := map[string]TraceNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for _, id := range seeds {
		if n, ok := byID[id]; ok && n.Kind == "pod" {
			return id
		}
	}
	if len(seeds) > 0 {
		return seeds[0]
	}
	return ""
}

func farthestNode(candidates []string, from string, adj map[string][]string) string {
	best := ""
	bestDist := -1
	for _, c := range candidates {
		if c == from {
			continue
		}
		path := bfsPath(from, c, adj)
		if len(path) < 2 {
			continue
		}
		if d := len(path); d > bestDist {
			bestDist = d
			best = c
		}
	}
	if best != "" {
		return best
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func bfsPath(start, goal string, adj map[string][]string) []string {
	if start == "" || goal == "" {
		return nil
	}
	if start == goal {
		return []string{start}
	}
	type step struct {
		id   string
		path []string
	}
	queue := []step{{id: start, path: []string{start}}}
	visited := map[string]struct{}{start: {}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur.id] {
			if _, ok := visited[next]; ok {
				continue
			}
			p := append(append([]string{}, cur.path...), next)
			if next == goal {
				return p
			}
			visited[next] = struct{}{}
			queue = append(queue, step{id: next, path: p})
		}
	}
	return nil
}

func edgeIDsOnPath(hops []string, edges []TraceEdge) []string {
	if len(hops) < 2 {
		return nil
	}
	out := []string{}
	for i := 0; i < len(hops)-1; i++ {
		a, b := hops[i], hops[i+1]
		for _, e := range edges {
			if (e.From == a && e.To == b) || (e.From == b && e.To == a) {
				out = append(out, e.ID)
				break
			}
		}
	}
	return out
}

func applyPathRefs(nodes []TraceNode, edges []TraceEdge, opts []PathOption, anchorID string) {
	pathByHop := map[string][]string{}
	pathByEdge := map[string][]string{}
	for _, o := range opts {
		for _, h := range o.HopIDs {
			pathByHop[h] = append(pathByHop[h], o.ID)
		}
		for _, eid := range o.EdgeIDs {
			pathByEdge[eid] = append(pathByEdge[eid], o.ID)
		}
	}
	for i := range nodes {
		n := &nodes[i]
		n.PathRefs = uniqueStrings(pathByHop[n.ID])
		if n.ID == anchorID {
			n.Track = "anchor"
			continue
		}
		n.Track = inferNodeTrack(n, opts)
	}
	for i := range edges {
		e := &edges[i]
		e.PathRefs = uniqueStrings(pathByEdge[e.ID])
		e.Direction = inferEdgeDirection(*e, nodes, anchorID)
	}
}

func inferNodeTrack(n *TraceNode, opts []PathOption) string {
	if len(n.PathRefs) == 0 {
		switch strings.ToLower(n.Kind) {
		case "networkpolicy", "nad", "bgp-peer":
			return "context"
		case "node", "ovn-logical-port", "bond":
			return "shared"
		}
		return "context"
	}
	ingress, egress, host := 0, 0, 0
	for _, ref := range n.PathRefs {
		for _, o := range opts {
			if o.ID != ref {
				continue
			}
			switch o.Direction {
			case PathIngress:
				ingress++
			case PathEgress:
				egress++
			case PathHost:
				host++
			}
		}
	}
	if ingress > 0 && egress == 0 {
		return "ingress"
	}
	if egress > 0 && ingress == 0 {
		return "egress"
	}
	if ingress > 0 && egress > 0 {
		return "shared"
	}
	if host > 0 {
		return "context"
	}
	return "shared"
}

func inferEdgeDirection(e TraceEdge, nodes []TraceNode, anchorID string) PathDirection {
	byID := map[string]TraceNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	src, dst := byID[e.From], byID[e.To]
	switch {
	case e.EdgeType == "egress", e.EdgeType == "egressservice":
		return PathEgress
	case e.EdgeType == "ingress", e.EdgeType == "https":
		return PathIngress
	case e.EdgeType == "policy-deny":
		return PathContext
	case src.Track == "ingress" || dst.Track == "ingress":
		return PathIngress
	case src.Track == "egress" || dst.Track == "egress":
		return PathEgress
	case e.From == anchorID:
		return PathEgress
	case e.To == anchorID:
		return PathIngress
	default:
		return PathHost
	}
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
