package graphdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/spcg/internal/graph/tenantcrypto"
	"github.com/netobserv/spcg/internal/pcap"
	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SigmaNode is the graph vertex payload for Sigma.js / Graphology.
type SigmaNode struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Size    float64 `json:"size"`
	Color   string  `json:"color"`
	Border  string  `json:"border,omitempty"`
	Tracked bool    `json:"tracked"`
	Type    string  `json:"type,omitempty"`
}

// SigmaEdge is the graph edge payload for Sigma.js / Graphology.
type SigmaEdge struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Label           string  `json:"label"`
	Color           string  `json:"color"`
	Size            float64 `json:"size"`
	TopologyEdgeID  string  `json:"topology_edge_id"`
	EdgeType        string  `json:"edge_type"`
	ExternalIP      string  `json:"external_ip,omitempty"`
	CountryCode     string  `json:"country_code,omitempty"`
}

// SigmaGraph is returned to the frontend for rendering.
type SigmaGraph struct {
	CaptureID   string                    `json:"capture_id"`
	Nodes       []SigmaNode               `json:"nodes"`
	Edges       []SigmaEdge               `json:"edges"`
	EdgeDetails map[string]pcap.EdgeDetail `json:"edge_details,omitempty"`
}

// Store persists per-capture topology subgraphs in Neo4j.
type Store struct {
	driver neo4j.DriverWithContext
	key    []byte // master secret; per-tenant keys derived per auth session
}

// Open connects to Neo4j when NEO4J_URI is set; otherwise returns a disabled store.
func Open(ctx context.Context) (*Store, error) {
	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		return &Store{}, nil
	}
	user := envOr("NEO4J_USER", "neo4j")
	pass := os.Getenv("NEO4J_PASSWORD")
	if pass == "" {
		return nil, fmt.Errorf("NEO4J_PASSWORD is required when NEO4J_URI is set")
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return nil, err
	}
	attempts := envInt("NEO4J_CONNECT_ATTEMPTS", 10)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := driver.VerifyConnectivity(ctx); err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		_ = driver.Close(ctx)
		if envBool("NEO4J_REQUIRED", false) {
			return nil, fmt.Errorf("neo4j connectivity: %w", lastErr)
		}
		log.Printf("neo4j connectivity failed after %d attempts, graph features disabled: %v", attempts, lastErr)
		return &Store{}, nil
	}
	master := os.Getenv("GRAPH_MASTER_KEY")
	return &Store{driver: driver, key: []byte(master)}, nil
}

func (s *Store) Enabled() bool {
	return s != nil && s.driver != nil
}

func (s *Store) Close(ctx context.Context) {
	if s.driver != nil {
		_ = s.driver.Close(ctx)
	}
}

func tenantKey(master []byte, authSessionID string) []byte {
	if len(master) == 0 {
		return nil
	}
	return tenantcrypto.KeyForTenant(string(master), authSessionID)
}

// ReplaceTopology wipes and rebuilds the capture subgraph (tenant-scoped).
func (s *Store) ReplaceTopology(ctx context.Context, captureID, authSessionID string, topo pcap.FlowTopology, trackedIDs map[string]bool) error {
	if !s.Enabled() {
		return nil
	}
	tkey := tenantKey(s.key, authSessionID)
	nodes, edges := layoutSigma(topo, trackedIDs)

	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		if _, err := tx.Run(ctx, `
			MATCH (n) WHERE n.captureId = $cid DETACH DELETE n
		`, map[string]any{"cid": captureID}); err != nil {
			return nil, err
		}
		if _, err := tx.Run(ctx, `
			MERGE (s:CaptureSession {captureId: $cid})
			SET s.authSessionId = $auth, s.updatedAt = datetime()
		`, map[string]any{"cid": captureID, "auth": authSessionID}); err != nil {
			return nil, err
		}
		for _, n := range nodes {
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
				"cid": captureID, "id": n.ID, "label": labelEnc, "kind": n.Type,
				"ns": namespaceFor(topo, n.ID), "tracked": n.Tracked, "color": n.Color, "border": n.Border,
				"x": n.X, "y": n.Y, "size": n.Size,
			})
			if err != nil {
				return nil, err
			}
		}
		for _, e := range edges {
			labelEnc, _ := tenantcrypto.Encrypt(tkey, e.Label)
			detailJSON := ""
			if d, ok := topo.EdgeDetail[e.TopologyEdgeID]; ok {
				if b, err := json.Marshal(d); err == nil {
					detailEnc, _ := tenantcrypto.Encrypt(tkey, string(b))
					detailJSON = detailEnc
				}
			}
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
				"cid": captureID, "src": e.Source, "dst": e.Target, "eid": e.TopologyEdgeID,
				"label": labelEnc, "health": e.EdgeType, "etype": e.EdgeType, "ext": e.ExternalIP,
				"detail": detailJSON,
			})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// GetSigmaGraph loads a capture subgraph for the UI.
func (s *Store) GetSigmaGraph(ctx context.Context, captureID, authSessionID string) (*SigmaGraph, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("neo4j graph store is not configured")
	}
	tkey := tenantKey(s.key, authSessionID)
	out := &SigmaGraph{CaptureID: captureID, EdgeDetails: map[string]pcap.EdgeDetail{}}

	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer sess.Close(ctx)

	_, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		rows, err := tx.Run(ctx, `
			MATCH (s:CaptureSession {captureId: $cid, authSessionId: $auth})-[:HAS_ENDPOINT]->(e:Endpoint)
			RETURN e.nodeId AS id, e.labelEnc AS label, e.x AS x, e.y AS y, e.size AS size,
			       e.color AS color, e.border AS border, e.tracked AS tracked, e.kind AS kind
		`, map[string]any{"cid": captureID, "auth": authSessionID})
		if err != nil {
			return nil, err
		}
		for rows.Next(ctx) {
			rec := rows.Record()
			label, _ := tenantcrypto.Decrypt(tkey, strVal(rec.Values[1]))
			out.Nodes = append(out.Nodes, SigmaNode{
				ID: strVal(rec.Values[0]), Label: label,
				X: floatVal(rec.Values[2]), Y: floatVal(rec.Values[3]), Size: floatVal(rec.Values[4]),
				Color: strVal(rec.Values[5]), Border: strVal(rec.Values[6]),
				Tracked: boolVal(rec.Values[7]), Type: strVal(rec.Values[8]),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		rows2, err := tx.Run(ctx, `
			MATCH (a:Endpoint {captureId: $cid})-[r:FLOWS_TO]->(b:Endpoint {captureId: $cid})
			RETURN r.edgeId AS eid, a.nodeId AS src, b.nodeId AS dst, r.labelEnc AS label,
			       r.edgeType AS etype, r.externalIp AS ext, r.detailEnc AS detail
		`, map[string]any{"cid": captureID})
		if err != nil {
			return nil, err
		}
		for rows2.Next(ctx) {
			rec := rows2.Record()
			eid := strVal(rec.Values[0])
			label, _ := tenantcrypto.Decrypt(tkey, strVal(rec.Values[3]))
			detailEnc := strVal(rec.Values[6])
			if detailEnc != "" {
				if plain, err := tenantcrypto.Decrypt(tkey, detailEnc); err == nil && plain != "" {
					var d pcap.EdgeDetail
					if json.Unmarshal([]byte(plain), &d) == nil {
						out.EdgeDetails[eid] = d
					}
				}
			}
			out.Edges = append(out.Edges, SigmaEdge{
				ID: eid, TopologyEdgeID: eid, Source: strVal(rec.Values[1]), Target: strVal(rec.Values[2]),
				Label: label, EdgeType: strVal(rec.Values[4]), ExternalIP: strVal(rec.Values[5]),
				Color: edgeColor(strVal(rec.Values[4]), ""), Size: 2,
			})
		}
		return nil, rows2.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteCapture removes all graph data for a capture session.
func (s *Store) DeleteCapture(ctx context.Context, captureID string) error {
	if !s.Enabled() {
		return nil
	}
	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)

	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `MATCH (n) WHERE n.captureId = $cid DETACH DELETE n`, map[string]any{"cid": captureID})
		if err != nil {
			return nil, err
		}
		_, err = tx.Run(ctx, `MATCH (s:CaptureSession {captureId: $cid}) DETACH DELETE s`, map[string]any{"cid": captureID})
		return nil, err
	})
	return err
}

// SigmaGraphFromTopology renders Sigma payloads without Neo4j round-trips.
func SigmaGraphFromTopology(captureID string, topo pcap.FlowTopology, tracked map[string]bool) *SigmaGraph {
	nodes, edges := layoutSigma(topo, tracked)
	return &SigmaGraph{
		CaptureID:   captureID,
		Nodes:       nodes,
		Edges:       edges,
		EdgeDetails: topo.EdgeDetail,
	}
}

func layoutSigma(topo pcap.FlowTopology, tracked map[string]bool) ([]SigmaNode, []SigmaEdge) {
	n := len(topo.Nodes)
	if n == 0 {
		return nil, nil
	}
	radius := 180.0 + float64(n)*8
	cx, cy := 0.0, 0.0
	nodes := make([]SigmaNode, 0, n)
	pos := map[string][2]float64{}
	for i, nd := range topo.Nodes {
		angle := (float64(i) / float64(n)) * 2 * math.Pi
		inner := 1.0
		if tracked[nd.ID] && len(tracked) <= 3 {
			inner = 0.55
		}
		x := cx + math.Cos(angle)*radius*inner
		y := cy + math.Sin(angle)*radius*inner
		pos[nd.ID] = [2]float64{x, y}
		size := 8.0
		if tracked[nd.ID] {
			size = 10
		}
		nodes = append(nodes, SigmaNode{
			ID: nd.ID, Label: nd.Label, X: x, Y: y, Size: size,
			Color: "#3b82f6", Border: "#94b4ff", Tracked: tracked[nd.ID], Type: nd.Kind,
		})
	}
	edges := make([]SigmaEdge, 0, len(topo.Edges))
	for _, e := range topo.Edges {
		if _, ok := pos[e.From]; !ok {
			continue
		}
		if _, ok := pos[e.To]; !ok {
			continue
		}
		ext := ""
		if len(e.From) > 4 && e.From[:4] == "ext/" {
			ext = e.From[4:]
		} else if len(e.To) > 4 && e.To[:4] == "ext/" {
			ext = e.To[4:]
		}
		etype := "direct"
		if e.Health == "dropped" {
			etype = "snat"
		} else if e.Health == "degraded" {
			etype = "scheduled"
		}
		label := e.Proto
		if e.DstPort > 0 {
			label += fmt.Sprintf(":%d", e.DstPort)
		}
		label += fmt.Sprintf(" · %d pkts", e.Count)
		edges = append(edges, SigmaEdge{
			ID: e.ID, TopologyEdgeID: e.ID, Source: e.From, Target: e.To,
			Label: label, EdgeType: etype, ExternalIP: ext,
			Color: edgeColor(etype, ""), Size: math.Max(1.5, math.Log2(float64(e.Packets+1))),
		})
	}
	return nodes, edges
}

func edgeColor(edgeType, country string) string {
	switch edgeType {
	case "snat":
		return "#fb7185"
	case "scheduled":
		return "#fbbf24"
	default:
		return "#7dd3fc"
	}
}

func namespaceFor(topo pcap.FlowTopology, id string) string {
	for _, n := range topo.Nodes {
		if n.ID == id {
			return n.Namespace
		}
	}
	return ""
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func strVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func floatVal(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func boolVal(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// LogDisabled logs once when graph store is off.
func LogDisabled() {
	log.Printf("neo4j graph store disabled (set NEO4J_URI to enable)")
}
