package trace

// layerForKind classifies nodes for logical vs physical graph overlays.
func layerForKind(kind string) string {
	switch kind {
	case "node", "bond", "host-veth", "vrf", "nad", "ovn-logical-port", "bgp-peer", "metallb-advertisement":
		return "physical"
	default:
		return "logical"
	}
}

// simplifyGraph keeps endpoint nodes and the focused path; drops context-only artefacts.
func simplifyGraph(g TraceGraph) TraceGraph {
	if len(g.Nodes) == 0 {
		return g
	}
	total := len(g.Nodes)
	keep := map[string]struct{}{}
	for _, n := range g.Nodes {
		if n.Tracked || n.Focused {
			keep[n.ID] = struct{}{}
		}
	}
	for _, e := range g.Edges {
		if !e.Primary {
			continue
		}
		keep[e.From] = struct{}{}
		keep[e.To] = struct{}{}
	}

	filteredNodes := make([]TraceNode, 0, len(keep))
	for _, n := range g.Nodes {
		if _, ok := keep[n.ID]; !ok {
			continue
		}
		if n.Layer == "" {
			n.Layer = layerForKind(n.Kind)
		}
		filteredNodes = append(filteredNodes, n)
	}

	filteredEdges := make([]TraceEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		if _, ok := keep[e.From]; !ok {
			continue
		}
		if _, ok := keep[e.To]; !ok {
			continue
		}
		filteredEdges = append(filteredEdges, e)
	}

	filteredPaths := make([]PathSummary, 0, len(g.Paths))
	for _, p := range g.Paths {
		if p.Status != "discovered" {
			continue
		}
		filteredPaths = append(filteredPaths, p)
	}

	layout, lanes := applyLayout(filteredNodes, filteredEdges, filteredPaths)
	layout.Edges = filteredEdges
	layout.Paths = filteredPaths
	layout.Stats = graphStats(total, layout.Nodes)
	layout.Lanes = lanes
	return layout
}

func graphStats(total int, nodes []TraceNode) *TraceGraphStats {
	st := &TraceGraphStats{TotalNodes: total, PrunedNodes: total - len(nodes)}
	for _, n := range nodes {
		if n.Focused || n.Tracked {
			st.FocusedNodes++
		}
		switch n.Layer {
		case "physical":
			st.PhysicalNodes++
		default:
			st.LogicalNodes++
		}
	}
	return st
}
