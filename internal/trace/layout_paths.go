package trace

import "strings"

const (
	trackIngress = "ingress"
	trackEgress  = "egress"
	trackAnchor  = "anchor"
	trackShared  = "shared"
	trackContext = "context"
)

// applyPathLayout positions nodes in ingress/egress swimlanes around the anchor workload.
func applyPathLayout(nodes []TraceNode, opts []PathOption, anchorID string) (TraceGraph, []TraceLane) {
	if len(nodes) == 0 {
		return TraceGraph{Width: 800, Height: 320}, nil
	}
	if anchorID == "" || len(opts) == 0 {
		return applyLayout(nodes)
	}

	ingressOpts := filterPathOptions(opts, PathIngress)
	egressOpts := filterPathOptions(opts, PathEgress)

	ingressRows := map[string]int{}
	for i, o := range ingressOpts {
		ingressRows[o.ID] = i
	}
	egressRows := map[string]int{}
	for i, o := range egressOpts {
		egressRows[o.ID] = i
	}

	anchorX := float64(padX + 4*(nodeW+36))
	anchorY := float64(padY + 180)

	for i := range nodes {
		n := &nodes[i]
		n.Width = float64(nodeW)
		n.Height = float64(nodeH)

		if n.ID == anchorID {
			n.X = anchorX
			n.Y = anchorY
			n.Track = trackAnchor
			continue
		}

		track := n.Track
		if track == "" {
			track = inferNodeTrack(n, opts)
			n.Track = track
		}

		switch track {
		case trackContext:
			idx := contextIndex(n.ID)
			n.X = float64(padX + idx*130)
			n.Y = float64(padY + 420)
		case trackIngress:
			row := firstPathRow(n.PathRefs, ingressRows)
			depth := hopDepth(n.ID, ingressOpts, anchorID, true)
			n.X = float64(padX + depth*(nodeW+36))
			n.Y = float64(padY + 40 + row*(nodeH+28))
		case trackEgress:
			row := firstPathRow(n.PathRefs, egressRows)
			depth := hopDepth(n.ID, egressOpts, anchorID, false)
			n.X = anchorX + 72 + float64(depth*(nodeW+32))
			n.Y = float64(padY + 300 + row*(nodeH+28))
		case trackShared:
			depth := sharedDepth(n.ID, ingressOpts, egressOpts, anchorID)
			n.X = anchorX - 40 + float64(depth*48)
			n.Y = anchorY + float64(nodeH+24)
		default:
			n.X = float64(padX)
			n.Y = float64(padY + 360)
		}
	}

	width := anchorX + float64(6*(nodeW+32)) + float64(padX)
	height := float64(padY + 520)
	lanes := []TraceLane{
		{Label: "Ingress paths", Rank: 0, X: float64(padX), Width: width - float64(padX*2)},
		{Label: "Anchor workload", Rank: 1, X: anchorX - 16, Width: float64(nodeW + 48)},
		{Label: "Egress paths", Rank: 2, X: anchorX, Width: width - anchorX - float64(padX)},
		{Label: "Context", Rank: 3, X: float64(padX), Width: width - float64(padX*2)},
	}
	return TraceGraph{Nodes: nodes, Width: width, Height: height, Lanes: lanes}, lanes
}

func filterPathOptions(opts []PathOption, dir PathDirection) []PathOption {
	out := make([]PathOption, 0)
	for _, o := range opts {
		if o.Direction == dir {
			out = append(out, o)
		}
	}
	return out
}

func firstPathRow(refs []string, rows map[string]int) int {
	best := 0
	found := false
	for _, ref := range refs {
		if r, ok := rows[ref]; ok {
			if !found || r < best {
				best = r
				found = true
			}
		}
	}
	return best
}

func hopDepth(nodeID string, opts []PathOption, anchorID string, ingress bool) int {
	best := 0
	for _, o := range opts {
		for i, hop := range o.HopIDs {
			if hop != nodeID {
				continue
			}
			depth := i
			if !ingress {
				if anchorIdx := indexOf(o.HopIDs, anchorID); anchorIdx >= 0 {
					depth = i - anchorIdx
				}
			}
			if depth > best {
				best = depth
			}
		}
	}
	return best
}

func sharedDepth(nodeID string, ingressOpts, egressOpts []PathOption, anchorID string) int {
	d := hopDepth(nodeID, ingressOpts, anchorID, true)
	if d == 0 {
		d = hopDepth(nodeID, egressOpts, anchorID, false)
	}
	return d
}

func indexOf(xs []string, target string) int {
	for i, x := range xs {
		if x == target {
			return i
		}
	}
	return -1
}

func contextIndex(id string) int {
	order := []string{"netpol", "nad", "bgp-peer"}
	for i, prefix := range order {
		if strings.HasPrefix(id, prefix) {
			return i
		}
	}
	return 3
}
