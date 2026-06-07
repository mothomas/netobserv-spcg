package graphdb

// Topology Cypher schema for cross-layer trace state.
//
// Node labels:
//   (:Pod), (:Namespace), (:Service), (:EgressIP), (:NetworkAttachmentDefinition),
//   (:LogicalRouter), (:LogicalSwitch), (:OVS_Bridge), (:Node), (:NMStateConfig),
//   (:Interface), (:BGP_Peer), (:LoadBalancer)
//
// Relationships:
//   CONSUMES, SCHEDULED_ON, ATTACHED_TO, MANAGED_BY, ADVERTISED_VIA,
//   CONFIGURED_BY, BINDS_TO, ENCAPSULATED_VIA, CONNECTS, ROUTES_VIA

const (
	CypherMergeTraceSession = `
		MERGE (s:TraceSession {traceId: $traceId})
		SET s.authSessionId = $auth,
		    s.updatedAt = datetime(),
		    s.layerCount = $layerCount
	`

	CypherClearTraceTopology = `
		MATCH (n)
		WHERE n.traceId = $traceId
		DETACH DELETE n
	`

	CypherMergePod = `
		MERGE (p:Pod {traceId: $traceId, id: $id})
		SET p.label = $label,
		    p.namespace = $namespace,
		    p.layer = $layer,
		    p.kind = $kind,
		    p.x = $x,
		    p.y = $y
		WITH p
		MATCH (s:TraceSession {traceId: $traceId})
		MERGE (s)-[:HAS_NODE]->(p)
	`

	CypherMergeGenericNode = `
		MERGE (n:%s {traceId: $traceId, id: $id})
		SET n.label = $label,
		    n.namespace = $namespace,
		    n.layer = $layer,
		    n.kind = $kind,
		    n.detail = $detail,
		    n.sensitive = $sensitive,
		    n.x = $x,
		    n.y = $y
		WITH n
		MATCH (s:TraceSession {traceId: $traceId})
		MERGE (s)-[:HAS_NODE]->(n)
	`

	CypherMergeEdge = `
		MATCH (a {traceId: $traceId, id: $from})
		MATCH (b {traceId: $traceId, id: $to})
		MERGE (a)-[r:%s {traceId: $traceId, id: $edgeId}]->(b)
		SET r.label = $label,
		    r.layer = $layer,
		    r.primary = $primary,
		    r.state = $state,
		    r.openflowCookie = $cookie,
		    r.aclMetadata = $acl
	`

	// Concrete relationship templates used by UpsertTopology.
	CypherPodConsumesService = `
		MATCH (p:Pod {traceId: $traceId, id: $podId})
		MATCH (svc:Service {traceId: $traceId, id: $svcId})
		MERGE (p)-[:CONSUMES {traceId: $traceId}]->(svc)
	`

	CypherPodScheduledOnNode = `
		MATCH (p:Pod {traceId: $traceId, id: $podId})
		MATCH (n:Node {traceId: $traceId, id: $nodeId})
		MERGE (p)-[:SCHEDULED_ON {traceId: $traceId}]->(n)
	`

	CypherPodAttachedToNAD = `
		MATCH (p:Pod {traceId: $traceId, id: $podId})
		MATCH (nad:NetworkAttachmentDefinition {traceId: $traceId, id: $nadId})
		MERGE (p)-[r:ATTACHED_TO {traceId: $traceId}]->(nad)
		SET r.interface = $iface, r.ip = $ip
	`

	CypherServiceManagedByLB = `
		MATCH (svc:Service {traceId: $traceId, id: $svcId})
		MATCH (lb:LoadBalancer {traceId: $traceId, id: $lbId})
		MERGE (svc)-[:MANAGED_BY {traceId: $traceId, provider: "metallb"}]->(lb)
	`

	CypherLBAdvertisedViaBGP = `
		MATCH (lb:LoadBalancer {traceId: $traceId, id: $lbId})
		MATCH (peer:BGP_Peer {traceId: $traceId, id: $peerId})
		MERGE (lb)-[:ADVERTISED_VIA {traceId: $traceId}]->(peer)
	`

	CypherNodeConfiguredByNMState = `
		MATCH (n:Node {traceId: $traceId, id: $nodeId})
		MATCH (cfg:NMStateConfig {traceId: $traceId, id: $cfgId})
		MERGE (n)-[:CONFIGURED_BY {traceId: $traceId}]->(cfg)
	`

	CypherNMStateBindsInterface = `
		MATCH (cfg:NMStateConfig {traceId: $traceId, id: $cfgId})
		MATCH (i:Interface {traceId: $traceId, id: $ifaceId})
		MERGE (cfg)-[:BINDS_TO {traceId: $traceId}]->(i)
	`

	CypherBridgeEncapsulatedVia = `
		MATCH (a:OVS_Bridge {traceId: $traceId, id: $fromId})
		MATCH (b:OVS_Bridge {traceId: $traceId, id: $toId})
		MERGE (a)-[:ENCAPSULATED_VIA {traceId: $traceId, vni: $vni, type: "geneve"}]->(b)
	`

	CypherUpdateEdgeState = `
		MATCH ()-[r {traceId: $traceId, id: $edgeId}]-()
		SET r.state = $state,
		    r.aclMetadata = $acl,
		    r.updatedAt = datetime()
	`
)

// SanitizeLabel ensures Neo4j label tokens are safe for fmt.Sprintf in CypherMergeGenericNode.
func SanitizeNeo4jLabel(label string) string {
	switch label {
	case "Pod", "Namespace", "Service", "EgressIP", "NetworkAttachmentDefinition",
		"LogicalRouter", "LogicalSwitch", "OVS_Bridge", "Node", "NMStateConfig",
		"Interface", "BGP_Peer", "LoadBalancer":
		return label
	default:
		return "Pod"
	}
}
