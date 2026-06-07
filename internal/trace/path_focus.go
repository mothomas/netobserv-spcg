package trace

// markFocusPath highlights the shortest route from any source endpoint to any destination endpoint.
func markFocusPath(b *graphBuilder) {
	if len(b.sourceIDs) == 0 || len(b.destIDs) == 0 {
		return
	}
	adj := map[string][]string{}
	edgeByKey := map[string]TraceEdge{}
	for _, e := range b.edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
		key := e.From + "->" + e.To
		edgeByKey[key] = e
		rev := e.To + "->" + e.From
		edgeByKey[rev] = e
	}

	type step struct {
		id   string
		path []string
	}
	queue := make([]step, 0, len(b.sourceIDs))
	visited := map[string]struct{}{}
	for id := range b.sourceIDs {
		if !b.hasNode(id) {
			continue
		}
		queue = append(queue, step{id: id, path: []string{id}})
		visited[id] = struct{}{}
	}

	var best []string
	for len(queue) > 0 && best == nil {
		cur := queue[0]
		queue = queue[1:]
		if _, ok := b.destIDs[cur.id]; ok {
			best = cur.path
			break
		}
		for _, next := range adj[cur.id] {
			if _, ok := visited[next]; ok {
				continue
			}
			visited[next] = struct{}{}
			p := append(append([]string{}, cur.path...), next)
			queue = append(queue, step{id: next, path: p})
		}
	}
	if len(best) < 2 {
		// Fallback: mark source/dest endpoints and direct edges touching them.
		for i := range b.nodes {
			if _, src := b.sourceIDs[b.nodes[i].ID]; src {
				b.nodes[i].Focused = true
			}
			if _, dst := b.destIDs[b.nodes[i].ID]; dst {
				b.nodes[i].Focused = true
			}
		}
		for i := range b.edges {
			if _, ok := b.sourceIDs[b.edges[i].From]; ok {
				if _, ok2 := b.destIDs[b.edges[i].To]; ok2 {
					b.edges[i].Primary = true
				}
			}
		}
		return
	}

	onPath := map[string]struct{}{}
	for _, id := range best {
		onPath[id] = struct{}{}
	}
	for i := range b.nodes {
		if _, ok := onPath[b.nodes[i].ID]; ok {
			b.nodes[i].Focused = true
		}
	}
	for i := 0; i < len(best)-1; i++ {
		a, c := best[i], best[i+1]
		for j := range b.edges {
			e := &b.edges[j]
			if (e.From == a && e.To == c) || (e.From == c && e.To == a) {
				e.Primary = true
			}
		}
	}
}
