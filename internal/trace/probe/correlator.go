package probe

import (
	"sort"
	"sync"

	"github.com/netobserv/spcg/internal/trace"
)

// GraphCorrelator maps probe observations onto primary trace graph edges.
type GraphCorrelator struct {
	mu         sync.RWMutex
	edges      []trace.TraceEdge
	edgeStates map[string]EdgePaintState
	seq        int
}

func NewGraphCorrelator(g trace.TraceGraph) *GraphCorrelator {
	edges := primaryEdgesInOrder(g)
	states := make(map[string]EdgePaintState, len(g.Edges))
	for _, e := range g.Edges {
		states[e.ID] = EdgeTheoryOnly
	}
	return &GraphCorrelator{edges: edges, edgeStates: states}
}

func primaryEdgesInOrder(g trace.TraceGraph) []trace.TraceEdge {
	rank := make(map[string]int, len(g.Nodes))
	for _, n := range g.Nodes {
		rank[n.ID] = n.Rank
	}
	var out []trace.TraceEdge
	for _, e := range g.Edges {
		if e.Primary {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := rank[out[i].From], rank[out[j].From]
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Advance marks the next primary edge active (simulate / sequential paint).
func (c *GraphCorrelator) Advance(hook string) (edgeID string, ok bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seq >= len(c.edges) {
		return "", false
	}
	edge := c.edges[c.seq]
	c.edgeStates[edge.ID] = EdgeActiveGreen
	c.seq++
	return edge.ID, true
}

// MarkDrop paints the last active or final edge as dropped.
// MarkDropOnEdge paints a specific edge as dropped.
func (c *GraphCorrelator) MarkDropOnEdge(edgeID string) bool {
	if c == nil || edgeID == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.edgeStates[edgeID]; !ok {
		return false
	}
	c.edgeStates[edgeID] = EdgeDroppedRed
	for i, e := range c.edges {
		if e.ID == edgeID {
			if c.seq <= i {
				c.seq = i + 1
			}
			break
		}
	}
	return true
}

// Remaining returns primary edges not yet advanced.
func (c *GraphCorrelator) Remaining() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.seq >= len(c.edges) {
		return 0
	}
	return len(c.edges) - c.seq
}

// NextEdgeID returns the next primary edge id without advancing.
func (c *GraphCorrelator) NextEdgeID() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.seq >= len(c.edges) {
		return ""
	}
	return c.edges[c.seq].ID
}

// VerifiedCount returns the number of primary edges painted active or dropped.
func (c *GraphCorrelator) VerifiedCount() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := 0
	for _, e := range c.edges {
		st := c.edgeStates[e.ID]
		if st == EdgeActiveGreen || st == EdgeDroppedRed {
			n++
		}
	}
	return n
}

func (c *GraphCorrelator) MarkDrop() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var edgeID string
	if c.seq > 0 {
		edgeID = c.edges[c.seq-1].ID
	} else if len(c.edges) > 0 {
		edgeID = c.edges[len(c.edges)-1].ID
	}
	if edgeID != "" {
		c.edgeStates[edgeID] = EdgeDroppedRed
	}
	return edgeID
}

func (c *GraphCorrelator) Snapshot() map[string]EdgePaintState {
	if c == nil {
		return map[string]EdgePaintState{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]EdgePaintState, len(c.edgeStates))
	for k, v := range c.edgeStates {
		out[k] = v
	}
	return out
}

func (c *GraphCorrelator) PrimaryCount() int {
	if c == nil {
		return 0
	}
	return len(c.edges)
}
