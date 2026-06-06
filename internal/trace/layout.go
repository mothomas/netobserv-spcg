package trace

const (
	nodeW = 148
	nodeH = 72
	padX  = 24
	padY  = 48
	rankGap = 96
)

var rankLaneLabel = map[int]string{
	0: "Source",
	1: "Ingress",
	2: "Mid",
	3: "Host / CNI",
	4: "Egress",
	5: "Destination",
}

// rankForKind maps node kind to cop timeline rank.
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
	case "egressip", "egressservice", "egress-router", "bgp-peer":
		return 4
	default:
		return 2
	}
}

type layoutNode struct {
	id   string
	kind string
}

// applyLayout assigns x/y to nodes by rank and sibling order.
func applyLayout(nodes []TraceNode) (TraceGraph, []TraceLane) {
	if len(nodes) == 0 {
		return TraceGraph{Width: 800, Height: 320}, nil
	}

	byRank := map[int][]int{}
	maxPerRank := 0
	for i, n := range nodes {
		r := n.Rank
		if r == 0 {
			r = rankForKind(n.Kind)
			nodes[i].Rank = r
		}
		byRank[r] = append(byRank[r], i)
		if len(byRank[r]) > maxPerRank {
			maxPerRank = len(byRank[r])
		}
	}

	ranks := make([]int, 0, len(byRank))
	for r := range byRank {
		ranks = append(ranks, r)
	}
	sortInts(ranks)

	laneX := float64(padX)
	lanes := make([]TraceLane, 0, len(ranks))
	maxX := laneX

	for _, r := range ranks {
		idxs := byRank[r]
		laneW := float64(len(idxs))*float64(nodeW) + float64(max(0, len(idxs)-1))*28
		if laneW < float64(nodeW) {
			laneW = float64(nodeW)
		}
		lanes = append(lanes, TraceLane{
			Label: rankLaneLabel[r],
			Rank:  r,
			X:     laneX,
			Width: laneW + 32,
		})
		for j, idx := range idxs {
			nodes[idx].X = laneX + 16 + float64(j)*(float64(nodeW)+28)
			row := (j - (len(idxs) - 1) / 2)
			nodes[idx].Y = float64(padY) + float64(row)*float64(nodeH+36)
			nodes[idx].Width = float64(nodeW)
			nodes[idx].Height = float64(nodeH)
		}
		laneX += laneW + float64(rankGap)
		if laneX > maxX {
			maxX = laneX
		}
	}

	height := float64(padY*2 + nodeH)
	if maxPerRank > 1 {
		height = float64(padY*2 + maxPerRank*(nodeH+36))
	}
	return TraceGraph{
		Nodes: nodes,
		Width: maxX + float64(padX),
		Height: height,
		Lanes: lanes,
	}, lanes
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
