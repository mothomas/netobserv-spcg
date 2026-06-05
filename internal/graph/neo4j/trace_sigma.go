package graphdb

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/netobserv/spcg/internal/graph/tenantcrypto"
	"github.com/netobserv/spcg/internal/trace"
	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SigmaGraphFromTrace maps infrastructure trace layout to the same Sigma payload as capture flows.
func SigmaGraphFromTrace(traceID string, g trace.TraceGraph) *SigmaGraph {
	nodes := make([]SigmaNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		color, border := traceNodeColors(n.Kind, n.Tracked)
		size := 10.0
		if n.Tracked {
			size = 14
		}
		if n.Width > 0 {
			size = math.Max(size, n.Width/14)
		}
		nodes = append(nodes, SigmaNode{
			ID:      n.ID,
			Label:   n.Label,
			X:       n.X + n.Width/2,
			Y:       n.Y + n.Height/2,
			Size:    size,
			Color:   color,
			Border:  border,
			Tracked: n.Tracked,
			Type:    n.Kind,
		})
	}
	edges := make([]SigmaEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		etype := traceEdgeType(e)
		label := e.Label
		if label == "" {
			label = e.EdgeType
		}
		edges = append(edges, SigmaEdge{
			ID:             e.ID,
			TopologyEdgeID: e.ID,
			Source:         e.From,
			Target:         e.To,
			Label:          label,
			EdgeType:       etype,
			Color:          edgeColor(etype, ""),
			Size:           traceEdgeSize(e),
		})
	}
	return &SigmaGraph{
		CaptureID: traceID,
		Nodes:     nodes,
		Edges:     edges,
	}
}

func traceEdgeType(e trace.TraceEdge) string {
	if e.Drop {
		return "snat"
	}
	if e.Primary {
		return "direct"
	}
	switch e.EdgeType {
	case "egressservice", "egress", "https":
		return e.EdgeType
	default:
		return "scheduled"
	}
}

func traceEdgeSize(e trace.TraceEdge) float64 {
	if e.Primary {
		return 2.5
	}
	return 1.5
}

func traceNodeColors(kind string, tracked bool) (color, border string) {
	if tracked {
		return "#60cdff", "#94b4ff"
	}
	switch strings.ToLower(kind) {
	case "pod":
		return "#3b82f6", "#94b4ff"
	case "route", "ingress":
		return "#22c55e", "#16a34a"
	case "service-clusterip", "service-loadbalancer", "service-nodeport", "service":
		return "#10b981", "#059669"
	case "egressip":
		return "#f97316", "#ea580c"
	case "egressservice":
		return "#7c3aed", "#6d28d9"
	case "egress-router":
		return "#c2410c", "#9a3412"
	case "loadbalancer-external", "metallb-pool", "bgp-peer":
		return "#14b8a6", "#0d9488"
	case "external-client", "external":
		return "#ef4444", "#dc2626"
	case "networkpolicy":
		return "#94a3b8", "#64748b"
	case "nad", "ovn-logical-port":
		return "#8b5cf6", "#7c3aed"
	case "node", "bond", "host-veth", "vrf":
		return "#64748b", "#475569"
	default:
		return "#3b82f6", "#64748b"
	}
}

// ReplaceTraceGraph persists trace infrastructure graph using the same Neo4j model as capture.
func (s *Store) ReplaceTraceGraph(ctx context.Context, traceID, authSessionID string, g trace.TraceGraph) error {
	if !s.Enabled() {
		return nil
	}
	sigma := SigmaGraphFromTrace(traceID, g)
	tkey := tenantKey(s.key, authSessionID)
	nsByID := map[string]string{}
	for _, n := range g.Nodes {
		nsByID[n.ID] = n.Namespace
	}

	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		if _, err := tx.Run(ctx, `MATCH (n) WHERE n.captureId = $cid DETACH DELETE n`, map[string]any{"cid": traceID}); err != nil {
			return nil, err
		}
		if _, err := tx.Run(ctx, `
			MERGE (s:CaptureSession {captureId: $cid})
			SET s.authSessionId = $auth, s.updatedAt = datetime(), s.traceSession = true
		`, map[string]any{"cid": traceID, "auth": authSessionID}); err != nil {
			return nil, err
		}
		for _, n := range sigma.Nodes {
			labelEnc, _ := tenantcrypto.Encrypt(tkey, n.Label)
			_, err := tx.Run(ctx, `
				CREATE (e:Endpoint {
					captureId: $cid,
					nodeId: $id,
					labelEnc: $label,
					kind: $kind,
					namespace: $ns,
					tracked: $tracked,
					color: $color,
					border: $border,
					x: $x,
					y: $y,
					size: $size
				})
				WITH e
				MATCH (s:CaptureSession {captureId: $cid})
				MERGE (s)-[:HAS_ENDPOINT]->(e)
			`, map[string]any{
				"cid": traceID, "id": n.ID, "label": labelEnc, "kind": n.Type,
				"ns": nsByID[n.ID], "tracked": n.Tracked, "color": n.Color, "border": n.Border,
				"x": n.X, "y": n.Y, "size": n.Size,
			})
			if err != nil {
				return nil, err
			}
		}
		for _, e := range sigma.Edges {
			labelEnc, _ := tenantcrypto.Encrypt(tkey, e.Label)
			detailEnc, _ := tenantcrypto.Encrypt(tkey, traceEdgeDetailJSON(g, e.TopologyEdgeID))
			_, err := tx.Run(ctx, `
				MATCH (a:Endpoint {captureId: $cid, nodeId: $src})
				MATCH (b:Endpoint {captureId: $cid, nodeId: $dst})
				CREATE (a)-[r:FLOWS_TO {
					captureId: $cid,
					edgeId: $eid,
					labelEnc: $label,
					health: $health,
					edgeType: $etype,
					externalIp: $ext,
					detailEnc: $detail
				}]->(b)
			`, map[string]any{
				"cid": traceID, "src": e.Source, "dst": e.Target, "eid": e.TopologyEdgeID,
				"label": labelEnc, "health": e.EdgeType, "etype": e.EdgeType, "ext": e.ExternalIP,
				"detail": detailEnc,
			})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func traceEdgeDetailJSON(g trace.TraceGraph, edgeID string) string {
	for _, e := range g.Edges {
		if e.ID != edgeID {
			continue
		}
		payload := map[string]interface{}{
			"edge_id":   e.ID,
			"edge_type": e.EdgeType,
			"primary":   e.Primary,
			"drop":      e.Drop,
			"label":     e.Label,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return ""
		}
		return string(b)
	}
	return ""
}

// GetTraceSigmaGraph loads a trace subgraph when Neo4j is enabled.
func (s *Store) GetTraceSigmaGraph(ctx context.Context, traceID, authSessionID string) (*SigmaGraph, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("neo4j graph store is not configured")
	}
	return s.GetSigmaGraph(ctx, traceID, authSessionID)
}
