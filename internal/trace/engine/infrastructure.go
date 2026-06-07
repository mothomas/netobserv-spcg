package engine

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// OVNClient queries OVN Southbound DB for chassis and port bindings.
type OVNClient interface {
	ChassisBindings(ctx context.Context, logicalPorts []string) ([]ChassisBinding, error)
	LogicalTopology(ctx context.Context, nodeNames []string) ([]LogicalSwitch, []LogicalRouter, error)
	ACLsForPorts(ctx context.Context, logicalPorts []string) ([]ACLHit, error)
}

// OVSFlowDumper returns ovs-ofctl dump-flows output for a bridge on a node.
type OVSFlowDumper interface {
	DumpFlows(ctx context.Context, nodeName, bridge string) (string, error)
}

type infrastructureDiscoverer struct {
	ovn OVNClient
	ovs OVSFlowDumper
}

func (d infrastructureDiscoverer) discover(ctx context.Context, logical LogicalPlaneResult, nodeNames []string) InfrastructurePlaneResult {
	out := InfrastructurePlaneResult{
		OVSBridges: defaultOVSBridges(nodeNames),
	}
	if d.ovn != nil {
		ports := logicalPortsFromMultus(logical.MultusAttachments)
		if bindings, err := d.ovn.ChassisBindings(ctx, ports); err == nil {
			out.ChassisBindings = bindings
		}
		if ls, lr, err := d.ovn.LogicalTopology(ctx, nodeNames); err == nil {
			out.LogicalSwitches = ls
			out.LogicalRouters = lr
		}
		if acls, err := d.ovn.ACLsForPorts(ctx, ports); err == nil {
			out.ACLHits = acls
		}
	}
	for _, node := range nodeNames {
		for _, bridge := range []string{"br-int", "br-ext", "br-local"} {
			if d.ovs == nil {
				continue
			}
			dump, err := d.ovs.DumpFlows(ctx, node, bridge)
			if err != nil || dump == "" {
				continue
			}
			rules := parseOpenFlowDump(node, bridge, dump)
			out.OpenFlowRules = append(out.OpenFlowRules, rules...)
		}
	}
	out.Nodes, out.Edges = infrastructureTopology(out)
	return out
}

func defaultOVSBridges(nodeNames []string) []OVSBridge {
	var out []OVSBridge
	for _, node := range nodeNames {
		for _, bridge := range []string{"br-int", "br-ext"} {
			out = append(out, OVSBridge{Name: bridge, NodeName: node})
		}
	}
	return out
}

func logicalPortsFromMultus(attachments []MultusAttachment) []string {
	out := make([]string, 0, len(attachments))
	for _, att := range attachments {
		if att.Interface != "" {
			out = append(out, att.PodNamespace+"_"+att.PodName+"_"+att.Interface)
		}
	}
	return out
}

var (
	reOpenFlowCookie = regexp.MustCompile(`cookie=([^,]+)`)
	reOpenFlowTable  = regexp.MustCompile(`table=(\d+)`)
	reGeneveVNI      = regexp.MustCompile(`set_field:(\d+)->tun_id`)
)

// parseOpenFlowDump parses ovs-ofctl dump-flows text into structured rules.
func parseOpenFlowDump(nodeName, bridge, dump string) []OpenFlowRule {
	var out []OpenFlowRule
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "NXST_FLOW") || strings.HasPrefix(line, "OFPST_FLOW") {
			continue
		}
		rule := OpenFlowRule{Bridge: bridge, NodeName: nodeName}
		if m := reOpenFlowCookie.FindStringSubmatch(line); len(m) > 1 {
			rule.Cookie = m[1]
		}
		if m := reOpenFlowTable.FindStringSubmatch(line); len(m) > 1 {
			rule.Table, _ = strconv.Atoi(m[1])
		}
		if idx := strings.Index(line, "actions="); idx >= 0 {
			rule.Match = strings.TrimSpace(line[:idx])
			rule.Actions = strings.TrimSpace(line[idx+len("actions="):])
		} else {
			rule.Match = line
		}
		rule.UsesCT = strings.Contains(line, "ct(") || strings.Contains(rule.Actions, "ct(")
		if m := reGeneveVNI.FindStringSubmatch(rule.Actions); len(m) > 1 {
			rule.GeneveVNI = m[1]
		}
		rule.Terminates = strings.Contains(rule.Actions, "drop") || strings.Contains(rule.Actions, "resubmit")
		out = append(out, rule)
	}
	return out
}

func infrastructureTopology(infra InfrastructurePlaneResult) ([]TopologyNode, []TopologyEdge) {
	nodes := make([]TopologyNode, 0, len(infra.OVSBridges)+len(infra.LogicalSwitches)+len(infra.LogicalRouters))
	edges := make([]TopologyEdge, 0)
	for _, ls := range infra.LogicalSwitches {
		id := fmt.Sprintf("ls:%s", ls.Name)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: ls.Name, Neo4jLabel: "LogicalSwitch", Layer: LayerInfrastructure,
			Shape: ShapeHexagon, Kind: "logical-switch", Detail: ls.Subnet, Sensitive: true,
		})
	}
	for _, lr := range infra.LogicalRouters {
		id := fmt.Sprintf("lr:%s", lr.Name)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: lr.Name, Neo4jLabel: "LogicalRouter", Layer: LayerInfrastructure,
			Shape: ShapeHexagon, Kind: "logical-router", Sensitive: true,
		})
	}
	for _, br := range infra.OVSBridges {
		id := fmt.Sprintf("ovs:%s:%s", br.NodeName, br.Name)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: br.Name, Neo4jLabel: "OVS_Bridge", Layer: LayerInfrastructure,
			Shape: ShapeHexagon, Kind: "ovs-bridge", Properties: map[string]string{"node": br.NodeName},
			Sensitive: true,
		})
	}
	for i, rule := range infra.OpenFlowRules {
		if rule.GeneveVNI == "" || !strings.Contains(rule.Bridge, "br-int") {
			continue
		}
		srcID := fmt.Sprintf("ovs:%s:br-int", rule.NodeName)
		dstID := fmt.Sprintf("ovs:%s:br-ext", rule.NodeName)
		edges = append(edges, TopologyEdge{
			ID: fmt.Sprintf("encap:%s:%d", rule.NodeName, i),
			From: srcID, To: dstID, RelType: "ENCAPSULATED_VIA", Layer: LayerInfrastructure,
			Label: "geneve", OpenFlowCookie: rule.Cookie,
			State: EdgeTheoryOnly,
		})
	}
	return nodes, edges
}

// StubOVNClient is a no-op OVN client used until ovn-sbdb connectivity is configured.
type StubOVNClient struct{}

func (StubOVNClient) ChassisBindings(context.Context, []string) ([]ChassisBinding, error) {
	return nil, nil
}
func (StubOVNClient) LogicalTopology(context.Context, []string) ([]LogicalSwitch, []LogicalRouter, error) {
	return nil, nil, nil
}
func (StubOVNClient) ACLsForPorts(context.Context, []string) ([]ACLHit, error) {
	return nil, nil
}

// SimulatedOVNClient synthesizes bindings from node names (dev/test without SBDB).
type SimulatedOVNClient struct{}

func (SimulatedOVNClient) ChassisBindings(_ context.Context, ports []string) ([]ChassisBinding, error) {
	out := make([]ChassisBinding, 0, len(ports))
	for _, p := range ports {
		out = append(out, ChassisBinding{LogicalPort: p, ChassisName: "chassis-sim", IfaceID: p})
	}
	return out, nil
}
func (SimulatedOVNClient) LogicalTopology(_ context.Context, nodeNames []string) ([]LogicalSwitch, []LogicalRouter, error) {
	ls := []LogicalSwitch{{Name: "ovn-cluster", Subnet: "10.128.0.0/14"}}
	lr := []LogicalRouter{{Name: "ovn-cluster-router"}}
	if len(nodeNames) > 0 {
		ls = append(ls, LogicalSwitch{Name: "join-" + nodeNames[0]})
	}
	return ls, lr, nil
}
func (SimulatedOVNClient) ACLsForPorts(context.Context, []string) ([]ACLHit, error) {
	return nil, nil
}

// StaticFlowDumper replays canned ovs-ofctl output for offline parsing tests.
type StaticFlowDumper map[string]string

func (s StaticFlowDumper) DumpFlows(_ context.Context, nodeName, bridge string) (string, error) {
	key := nodeName + "/" + bridge
	if v, ok := s[key]; ok {
		return v, nil
	}
	return "", nil
}
