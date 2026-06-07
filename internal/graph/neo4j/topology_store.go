package graphdb

import (
	"context"
	"fmt"

	"github.com/netobserv/spcg/internal/trace/engine"
	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// UpsertTopology persists a cross-layer TopologyResult into Neo4j using the relational schema.
func (s *Store) UpsertTopology(ctx context.Context, traceID, authSessionID string, topo *engine.TopologyResult) error {
	if !s.Enabled() || topo == nil {
		return nil
	}
	layerCount := 3
	return s.RunWrite(ctx, func(tx neo4j.ManagedTransaction) error {
		if _, err := tx.Run(ctx, CypherClearTraceTopology, map[string]any{"traceId": traceID}); err != nil {
			return err
		}
		if _, err := tx.Run(ctx, CypherMergeTraceSession, map[string]any{
			"traceId": traceID, "auth": authSessionID, "layerCount": layerCount,
		}); err != nil {
			return err
		}
		for _, n := range topo.Nodes {
			label := SanitizeNeo4jLabel(n.Neo4jLabel)
			query := fmt.Sprintf(CypherMergeGenericNode, label)
			if _, err := tx.Run(ctx, query, map[string]any{
				"traceId": traceID, "id": n.ID, "label": n.Label, "namespace": n.Namespace,
				"layer": string(n.Layer), "kind": n.Kind, "detail": n.Detail,
				"sensitive": n.Sensitive, "x": n.X, "y": n.Y,
			}); err != nil {
				return err
			}
		}
		for _, e := range topo.Edges {
			rel := sanitizeRelType(e.RelType)
			query := fmt.Sprintf(CypherMergeEdge, rel)
			state := string(e.State)
			if state == "" {
				state = string(engine.EdgeTheoryOnly)
			}
			if _, err := tx.Run(ctx, query, map[string]any{
				"traceId": traceID, "from": e.From, "to": e.To, "edgeId": e.ID,
				"label": e.Label, "layer": string(e.Layer), "primary": e.Primary,
				"state": state, "cookie": e.OpenFlowCookie, "acl": e.ACLMetadata,
			}); err != nil {
				return err
			}
		}
		return wireSemanticRelationships(ctx, tx, traceID, topo)
	})
}

func wireSemanticRelationships(ctx context.Context, tx neo4j.ManagedTransaction, traceID string, topo *engine.TopologyResult) error {
	for _, att := range topo.Logical.MultusAttachments {
		podID := fmt.Sprintf("pod:%s/%s", att.PodNamespace, att.PodName)
		nadID := fmt.Sprintf("nad:%s/%s:%s", att.NADNamespace, att.NADName, att.Interface)
		if _, err := tx.Run(ctx, CypherPodAttachedToNAD, map[string]any{
			"traceId": traceID, "podId": podID, "nadId": nadID, "iface": att.Interface, "ip": att.IP,
		}); err != nil {
			return err
		}
	}
	for _, route := range topo.Physical.HostRoutes {
		nodeID := fmt.Sprintf("node:%s", route.NodeName)
		cfgID := fmt.Sprintf("nmstate:%s", route.NodeName)
		if _, err := tx.Run(ctx, CypherNodeConfiguredByNMState, map[string]any{
			"traceId": traceID, "nodeId": nodeID, "cfgId": cfgID,
		}); err != nil {
			return err
		}
	}
	for _, edge := range topo.Infrastructure.Edges {
		if edge.RelType != "ENCAPSULATED_VIA" {
			continue
		}
		vni := "4096"
		if _, err := tx.Run(ctx, CypherBridgeEncapsulatedVia, map[string]any{
			"traceId": traceID, "fromId": edge.From, "toId": edge.To, "vni": vni,
		}); err != nil {
			return err
		}
	}
	return nil
}

// UpdateEdgeVerificationState mutates live eBPF correlation flags on a trace edge.
func (s *Store) UpdateEdgeVerificationState(ctx context.Context, traceID, edgeID, state, acl string) error {
	if !s.Enabled() {
		return nil
	}
	return s.RunWrite(ctx, func(tx neo4j.ManagedTransaction) error {
		_, err := tx.Run(ctx, CypherUpdateEdgeState, map[string]any{
			"traceId": traceID, "edgeId": edgeID, "state": state, "acl": acl,
		})
		return err
	})
}

func sanitizeRelType(rel string) string {
	switch rel {
	case "CONSUMES", "SCHEDULED_ON", "ATTACHED_TO", "MANAGED_BY", "ADVERTISED_VIA",
		"CONFIGURED_BY", "BINDS_TO", "ENCAPSULATED_VIA", "CONNECTS", "ROUTES_VIA":
		return rel
	default:
		return "CONNECTS"
	}
}
