package trace

const (
	nodeW       = 148
	nodeH       = 72
	ctxNodeW    = 128
	ctxNodeH    = 40
	padX        = 40
	padY        = 48
	hopGap      = 40
	ingressY    = 52
	anchorY     = 152
	egressY     = 252
	contextY    = 348
)

var trackLaneMeta = []struct {
	Track  string
	Label  string
	Y      float64
	Height float64
}{
	{Track: TrackIngress, Label: "Ingress paths", Y: ingressY - 20, Height: nodeH + 36},
	{Track: TrackAnchor, Label: "Workload", Y: anchorY - 20, Height: nodeH + 36},
	{Track: TrackShared, Label: "Host / CNI", Y: anchorY - 20, Height: nodeH + 36},
	{Track: TrackEgress, Label: "Egress paths", Y: egressY - 20, Height: nodeH + 36},
}

// rankForKind maps node kind to cop timeline rank (ordering within a track).
func rankForKind(kind string) int {
	switch kind {
	case "external-client", "external":
		return 0
	case "route", "ingress", "metallb-pool", "loadbalancer-external":
		return 1
	case "service-clusterip", "service-loadbalancer", "service-nodeport", "service", "pod", "nad":
		return 2
	case "node", "ovn-logical-port", "host-veth", "bond", "vrf", "networkpolicy":
		return 3
	case "egressip", "egressservice", "egress-router", "bgp-peer", "metallb-advertisement":
		return 4
	default:
		return 2
	}
}

// applyLayout places nodes on separate ingress / anchor / egress swimlanes.
func applyLayout(nodes []TraceNode, edges []TraceEdge, paths []PathSummary) (TraceGraph, []TraceLane) {
	if len(nodes) == 0 {
		return TraceGraph{Width: 960, Height: 420}, nil
	}

	buckets := map[string][]int{
		TrackIngress: {},
		TrackAnchor:  {},
		TrackShared:  {},
		TrackEgress:  {},
		TrackContext: {},
	}
	for i := range nodes {
		if nodes[i].Rank == 0 && nodes[i].Kind != "pod" {
			nodes[i].Rank = rankForKind(nodes[i].Kind)
		}
		track := nodes[i].Track
		if track == "" {
			track = inferPathTrack(nodes[i], edges, paths)
		}
		nodes[i].Track = track
		buckets[track] = append(buckets[track], i)
	}
	for track, idxs := range buckets {
		if track == TrackContext {
			continue
		}
		sortIndicesByRank(nodes, idxs)
	}

	ingressN := len(buckets[TrackIngress])
	anchorN := max(len(buckets[TrackAnchor]), 1)
	anchorX := padX + float64(max(ingressN, 1))*(nodeW+hopGap)

	for j, idx := range buckets[TrackIngress] {
		placeNode(&nodes[idx], padX+float64(j)*float64(nodeW+hopGap), ingressY, nodeW, nodeH)
	}
	for j, idx := range buckets[TrackAnchor] {
		placeNode(&nodes[idx], anchorX+float64(j)*float64(nodeW+16), anchorY, nodeW, nodeH)
	}

	sharedStart := anchorX + float64(anchorN)*(nodeW+16) + hopGap
	for j, idx := range buckets[TrackShared] {
		placeNode(&nodes[idx], sharedStart+float64(j)*float64(nodeW+hopGap), anchorY, nodeW, nodeH)
	}

	egressStart := sharedStart
	if len(buckets[TrackShared]) > 0 {
		egressStart += float64(len(buckets[TrackShared])) * float64(nodeW+hopGap)
	} else {
		egressStart = anchorX + float64(anchorN)*(nodeW+16) + hopGap
	}
	for j, idx := range buckets[TrackEgress] {
		placeNode(&nodes[idx], egressStart+float64(j)*float64(nodeW+hopGap), egressY, nodeW, nodeH)
	}

	maxX := egressStart
	if len(buckets[TrackEgress]) > 0 {
		maxX = egressStart + float64(len(buckets[TrackEgress]))*float64(nodeW+hopGap)
	} else if len(buckets[TrackShared]) > 0 {
		maxX = sharedStart + float64(len(buckets[TrackShared]))*float64(nodeW+hopGap)
	} else {
		maxX = anchorX + float64(anchorN)*float64(nodeW+16) + nodeW
	}

	for j, idx := range buckets[TrackContext] {
		col := j % 6
		row := j / 6
		placeNode(&nodes[idx], padX+float64(col)*float64(ctxNodeW+14), contextY+float64(row)*float64(ctxNodeH+12), ctxNodeW, ctxNodeH)
	}

	height := float64(egressY + nodeH + padY)
	if len(buckets[TrackContext]) > 0 {
		rows := (len(buckets[TrackContext]) + 5) / 6
		height = float64(contextY + rows*(ctxNodeH+12) + ctxNodeH + padY)
	}

	graphWidth := maxX + float64(padX)
	lanes := buildTrackLanes(graphWidth)

	return TraceGraph{
		Nodes:  nodes,
		Width:  graphWidth,
		Height: height,
		Lanes:  lanes,
	}, lanes
}

func placeNode(n *TraceNode, x, y float64, w, h int) {
	n.X = x
	n.Y = y
	n.Width = float64(w)
	n.Height = float64(h)
}

func buildTrackLanes(graphWidth float64) []TraceLane {
	lanes := make([]TraceLane, 0, 4)
	for i, meta := range trackLaneMeta {
		if meta.Track == TrackShared {
			continue
		}
		lanes = append(lanes, TraceLane{
			Label:  meta.Label,
			Rank:   i,
			X:      padX - 12,
			Width:  graphWidth - padX + 12,
			Y:      meta.Y,
			Height: meta.Height,
			Track:  meta.Track,
		})
	}
	return lanes
}

func sortIndicesByRank(nodes []TraceNode, idxs []int) {
	for i := 1; i < len(idxs); i++ {
		for j := i; j > 0 && nodes[idxs[j]].Rank < nodes[idxs[j-1]].Rank; j-- {
			idxs[j], idxs[j-1] = idxs[j-1], idxs[j]
		}
	}
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
