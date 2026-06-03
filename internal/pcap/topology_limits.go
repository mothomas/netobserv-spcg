package pcap

import "sort"

const (
	// MaxTopologyBuildEvents caps work per refresh under high packet rates.
	MaxTopologyBuildEvents = 2500
	MaxTopologyNodes       = 100
	MaxTopologyEdges       = 150
)

// SampleEventsForGraph keeps the most recent events for topology/graph builds.
func SampleEventsForGraph(events []FlowEvent, max int) []FlowEvent {
	if max <= 0 || len(events) <= max {
		return events
	}
	return events[len(events)-max:]
}

// LimitFlowTopology trims large graphs (many external peers under scan stress).
// Tracked pod nodes are always kept; top edges by bytes are retained.
func LimitFlowTopology(topo FlowTopology, tracked []TrackedPod, maxNodes, maxEdges int) (FlowTopology, bool) {
	if maxNodes <= 0 {
		maxNodes = MaxTopologyNodes
	}
	if maxEdges <= 0 {
		maxEdges = MaxTopologyEdges
	}
	capped := len(topo.Nodes) > maxNodes || len(topo.Edges) > maxEdges
	if !capped {
		return topo, false
	}

	trackedSet := trackedNodeIDs(tracked)
	nodeByID := map[string]TopologyNode{}
	for _, n := range topo.Nodes {
		nodeByID[n.ID] = n
	}

	edges := append([]TopologyEdge(nil), topo.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Bytes == edges[j].Bytes {
			return edges[i].Packets > edges[j].Packets
		}
		return edges[i].Bytes > edges[j].Bytes
	})
	if len(edges) > maxEdges {
		edges = edges[:maxEdges]
	}

	keepNodes := map[string]struct{}{}
	for id := range trackedSet {
		keepNodes[id] = struct{}{}
	}
	for _, e := range edges {
		keepNodes[e.From] = struct{}{}
		keepNodes[e.To] = struct{}{}
	}

	if len(keepNodes) > maxNodes {
		type nodeScore struct {
			id    string
			bytes uint64
		}
		scores := make([]nodeScore, 0, len(keepNodes))
		bytesByNode := map[string]uint64{}
		for _, e := range edges {
			bytesByNode[e.From] += e.Bytes
			bytesByNode[e.To] += e.Bytes
		}
		for id := range keepNodes {
			if _, tracked := trackedSet[id]; tracked {
				continue
			}
			scores = append(scores, nodeScore{id: id, bytes: bytesByNode[id]})
		}
		sort.Slice(scores, func(i, j int) bool { return scores[i].bytes > scores[j].bytes })
		nonTrackedBudget := maxNodes - len(trackedSet)
		if nonTrackedBudget < 0 {
			nonTrackedBudget = 0
		}
		allowed := map[string]struct{}{}
		for id := range trackedSet {
			allowed[id] = struct{}{}
		}
		for i := 0; i < len(scores) && i < nonTrackedBudget; i++ {
			allowed[scores[i].id] = struct{}{}
		}
		keepNodes = allowed
	}

	filteredEdges := make([]TopologyEdge, 0, len(edges))
	edgeDetail := map[string]EdgeDetail{}
	for _, e := range edges {
		if _, ok := keepNodes[e.From]; !ok {
			continue
		}
		if _, ok := keepNodes[e.To]; !ok {
			continue
		}
		filteredEdges = append(filteredEdges, e)
		if d, ok := topo.EdgeDetail[e.ID]; ok {
			edgeDetail[e.ID] = d
		}
	}

	nodes := make([]TopologyNode, 0, len(keepNodes))
	nsSet := map[string]struct{}{}
	for id := range keepNodes {
		if n, ok := nodeByID[id]; ok {
			nodes = append(nodes, n)
			if n.Namespace != "" {
				nsSet[n.Namespace] = struct{}{}
			}
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	return FlowTopology{
		Nodes:      nodes,
		Edges:      filteredEdges,
		Namespaces: namespaces,
		EdgeDetail: edgeDetail,
	}, true
}

// BuildBoundedTopology builds a filtered, size-limited topology for UI refresh.
func BuildBoundedTopology(events []FlowEvent, tracked []TrackedPod) (FlowTopology, bool) {
	events = SampleEventsForGraph(events, MaxTopologyBuildEvents)
	topo := BuildFlowTopology(events, tracked)
	if ids := trackedNodeIDs(tracked); len(ids) > 0 {
		topo = FilterTopologyToSelection(topo, ids)
	}
	topo, capped := LimitFlowTopology(topo, tracked, MaxTopologyNodes, MaxTopologyEdges)

	partial := FlowTopology{Nodes: topo.Nodes, Edges: topo.Edges, Namespaces: topo.Namespaces}
	details := map[string]EdgeDetail{}
	for _, e := range topo.Edges {
		if d, ok := topo.EdgeDetail[e.ID]; ok {
			details[e.ID] = d
		} else {
			details[e.ID] = EdgeDetail{EdgeID: e.ID, Sequence: ConversationSequence(partial, e.ID)}
		}
	}
	topo.EdgeDetail = details
	return topo, capped
}
