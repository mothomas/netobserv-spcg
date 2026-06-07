package engine

import (
	"github.com/netobserv/spcg/internal/trace"
)

// Endpoint is a trace source or destination target (alias for API stability).
type Endpoint = trace.TraceEndpoint

// EdgeVerificationState is live eBPF correlation state on a predicted hop.
type EdgeVerificationState string

const (
	EdgeTheoryOnly   EdgeVerificationState = "THEORY_ONLY"
	EdgeActiveGreen  EdgeVerificationState = "ACTIVE_GREEN"
	EdgeDroppedRed   EdgeVerificationState = "DROPPED_RED"
)

// LayerName identifies the three visualization regions for the frontend canvas.
type LayerName string

const (
	LayerLogical         LayerName = "logical"
	LayerInfrastructure  LayerName = "infrastructure"
	LayerPhysical        LayerName = "physical"
)

// VisualShape hints React Flow / Sigma node chrome.
type VisualShape string

const (
	ShapeRounded   VisualShape = "rounded"
	ShapeHexagon   VisualShape = "hexagon"
	ShapeRectangle VisualShape = "rectangle"
)

// TopologyNode is a relational graph vertex for Neo4j and UI layer metadata.
type TopologyNode struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Neo4jLabel  string            `json:"neo4j_label"`
	Layer       LayerName         `json:"layer"`
	Shape       VisualShape       `json:"shape"`
	Namespace   string            `json:"namespace,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	Sensitive   bool              `json:"sensitive,omitempty"`
	BoundingKey string            `json:"bounding_key,omitempty"`
	Rank        int               `json:"rank,omitempty"`
	X           float64           `json:"x,omitempty"`
	Y           float64           `json:"y,omitempty"`
}

// TopologyEdge connects topology nodes with verification and OpenFlow metadata.
type TopologyEdge struct {
	ID           string                `json:"id"`
	From         string                `json:"from"`
	To           string                `json:"to"`
	RelType      string                `json:"rel_type"`
	Layer        LayerName             `json:"layer"`
	Primary      bool                  `json:"primary,omitempty"`
	Label        string                `json:"label,omitempty"`
	OpenFlowCookie string              `json:"openflow_cookie,omitempty"`
	ACLMetadata  string                `json:"acl_metadata,omitempty"`
	State        EdgeVerificationState `json:"state"`
}

// LogicalPlaneResult is Phase 1 output (K8s + Multus + MetalLB + egress policy).
type LogicalPlaneResult struct {
	ResolvedSource      Endpoint            `json:"resolved_source"`
	ResolvedDestination Endpoint            `json:"resolved_destination"`
	MultusAttachments   []MultusAttachment  `json:"multus_attachments,omitempty"`
	MetalLBPaths        []MetalLBPath       `json:"metallb_paths,omitempty"`
	EgressBindings      []EgressBinding     `json:"egress_bindings,omitempty"`
	AdminPolicyRoutes   []AdminPolicyRoute  `json:"admin_policy_routes,omitempty"`
	Nodes               []TopologyNode      `json:"nodes"`
	Edges               []TopologyEdge      `json:"edges"`
}

// InfrastructurePlaneResult is Phase 2 output (OVN SB + OVS bridges/flows).
type InfrastructurePlaneResult struct {
	ChassisBindings []ChassisBinding `json:"chassis_bindings,omitempty"`
	LogicalSwitches []LogicalSwitch  `json:"logical_switches,omitempty"`
	LogicalRouters  []LogicalRouter  `json:"logical_routers,omitempty"`
	OVSBridges      []OVSBridge      `json:"ovs_bridges,omitempty"`
	OpenFlowRules   []OpenFlowRule   `json:"openflow_rules,omitempty"`
	ACLHits         []ACLHit         `json:"acl_hits,omitempty"`
	Nodes           []TopologyNode   `json:"nodes"`
	Edges           []TopologyEdge   `json:"edges"`
}

// PhysicalPlaneResult is Phase 3 output (NMState + host routing).
type PhysicalPlaneResult struct {
	HostRoutes      []HostRoute     `json:"host_routes,omitempty"`
	InterfaceMaps   []InterfaceMap  `json:"interface_maps,omitempty"`
	NMStateReports  []NMStateReport `json:"nmstate_reports,omitempty"`
	Nodes           []TopologyNode  `json:"nodes"`
	Edges           []TopologyEdge  `json:"edges"`
}

// TopologyResult is the full cross-layer trace output.
type TopologyResult struct {
	TraceID         string                      `json:"trace_id"`
	Source          Endpoint                    `json:"source"`
	Destination     Endpoint                    `json:"destination"`
	Graph           trace.TraceGraph            `json:"graph"`
	Logical         LogicalPlaneResult          `json:"logical"`
	Infrastructure  InfrastructurePlaneResult   `json:"infrastructure"`
	Physical        PhysicalPlaneResult         `json:"physical"`
	Nodes           []TopologyNode              `json:"nodes"`
	Edges           []TopologyEdge              `json:"edges"`
	EdgeStates      map[string]EdgeVerificationState `json:"edge_states"`
	Layers          VisualizationLayers         `json:"layers"`
}

// VisualizationLayers defines bounding regions for the frontend canvas.
type VisualizationLayers struct {
	Logical        LayerRegion `json:"logical"`
	Infrastructure LayerRegion `json:"infrastructure"`
	Physical       LayerRegion `json:"physical"`
}

// LayerRegion is a suggested viewport band for grouped rendering.
type LayerRegion struct {
	Label  string  `json:"label"`
	Anchor string  `json:"anchor"` // top-left | middle | bottom-right
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// MultusAttachment describes a secondary NAD path on a pod.
type MultusAttachment struct {
	PodNamespace string `json:"pod_namespace"`
	PodName      string `json:"pod_name"`
	Interface    string `json:"interface"`
	NADName      string `json:"nad_name"`
	NADNamespace string `json:"nad_namespace"`
	CNIType      string `json:"cni_type"`
	Topology     string `json:"topology,omitempty"`
	Subnets      string `json:"subnets,omitempty"`
	VLANID       string `json:"vlan_id,omitempty"`
	IPAMClaims   string `json:"ipam_claims,omitempty"`
	IP           string `json:"ip,omitempty"`
	HostBypass   bool   `json:"host_bypass,omitempty"`
	HostIface    string `json:"host_iface,omitempty"`
}

// MetalLBPath links a LoadBalancer Service to pool/ad/peer/speaker nodes.
type MetalLBPath struct {
	ServiceNamespace string   `json:"service_namespace"`
	ServiceName      string   `json:"service_name"`
	VIP              string   `json:"vip"`
	PoolName         string   `json:"pool_name"`
	Advertisements   []string `json:"advertisements,omitempty"`
	PeerNames        []string `json:"peer_names,omitempty"`
	SpeakerNodes     []string `json:"speaker_nodes,omitempty"`
}

// EgressBinding tracks OVN EgressIP or EgressService SNAT on a pod/node path.
type EgressBinding struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	EgressIP  string `json:"egress_ip,omitempty"`
	NodeName  string `json:"node_name,omitempty"`
}

// AdminPolicyRoute is an OVN AdminPolicyBasedExternalRoute reference.
type AdminPolicyRoute struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	NextHop   string `json:"next_hop,omitempty"`
}

// ChassisBinding maps a logical port to an OVN hypervisor chassis.
type ChassisBinding struct {
	LogicalPort string `json:"logical_port"`
	ChassisName string `json:"chassis_name"`
	ChassisUUID string `json:"chassis_uuid"`
	NodeName    string `json:"node_name,omitempty"`
	IfaceID     string `json:"iface_id,omitempty"`
}

// LogicalSwitch is an OVN logical switch (ls).
type LogicalSwitch struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid,omitempty"`
	Subnet  string `json:"subnet,omitempty"`
}

// LogicalRouter is an OVN logical router (lr).
type LogicalRouter struct {
	Name   string `json:"name"`
	UUID   string `json:"uuid,omitempty"`
	Policy string `json:"policy,omitempty"`
}

// OVSBridge represents br-int, br-ext, or br-local on a worker.
type OVSBridge struct {
	Name     string `json:"name"`
	NodeName string `json:"node_name"`
}

// OpenFlowRule is a parsed ovs-ofctl flow entry on a bridge.
type OpenFlowRule struct {
	Bridge     string `json:"bridge"`
	NodeName   string `json:"node_name"`
	Cookie     string `json:"cookie"`
	Table      int    `json:"table"`
	Match      string `json:"match"`
	Actions    string `json:"actions"`
	UsesCT     bool   `json:"uses_ct,omitempty"`
	GeneveVNI  string `json:"geneve_vni,omitempty"`
	Terminates bool   `json:"terminates,omitempty"`
}

// ACLHit is an OVN ACL applied to an ls/lr port.
type ACLHit struct {
	LogicalSwitch string `json:"logical_switch,omitempty"`
	LogicalRouter string `json:"logical_router,omitempty"`
	Direction     string `json:"direction"`
	Match         string `json:"match"`
	Action        string `json:"action"`
	Priority      int    `json:"priority"`
}

// HostRoute is a Linux routing table entry on a worker/gateway node.
type HostRoute struct {
	NodeName  string `json:"node_name"`
	Table     string `json:"table"`
	DestCIDR  string `json:"dest_cidr"`
	Gateway   string `json:"gateway,omitempty"`
	Dev       string `json:"dev,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

// InterfaceMap binds NMState/OVS bridge to physical NIC, VF, or bond.
type InterfaceMap struct {
	NodeName     string `json:"node_name"`
	Bridge       string `json:"bridge,omitempty"`
	LogicalPort  string `json:"logical_port,omitempty"`
	PhysicalName string `json:"physical_name"`
	Type         string `json:"type"` // ethernet, bond, vf, sriov
	MAC          string `json:"mac,omitempty"`
	State        string `json:"state,omitempty"`
}

// NMStateReport is a summarized NodeNetworkState interface report.
type NMStateReport struct {
	NodeName   string `json:"node_name"`
	ReportName string `json:"report_name"`
	Interfaces int    `json:"interfaces"`
}
