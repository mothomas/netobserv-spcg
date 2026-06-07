package engine

import (
	"context"
	"fmt"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gvrNMState = schema.GroupVersionResource{
	Group: "nmstate.io", Version: "v1", Resource: "nodenetworkstates",
}

type physicalDiscoverer struct {
	dc traceDynamicLister
}

func (d physicalDiscoverer) discover(ctx context.Context, resp *trace.DiscoverResponse, infra InfrastructurePlaneResult) PhysicalPlaneResult {
	out := PhysicalPlaneResult{}
	nodeNames := collectNodeNames(resp)
	if d.dc != nil {
		out.NMStateReports, out.InterfaceMaps = discoverNMState(ctx, d.dc, nodeNames, infra)
	}
	out.HostRoutes = synthesizeHostRoutes(nodeNames, resp)
	out.Nodes, out.Edges = physicalTopology(out, nodeNames)
	return out
}

func collectNodeNames(resp *trace.DiscoverResponse) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if resp != nil {
		for _, p := range resp.SourcePods {
			add(p.NodeName)
		}
		for _, p := range resp.DestPods {
			add(p.NodeName)
		}
	}
	return out
}

func discoverNMState(ctx context.Context, dc traceDynamicLister, nodeNames []string, infra InfrastructurePlaneResult) ([]NMStateReport, []InterfaceMap) {
	var reports []NMStateReport
	var ifmaps []InterfaceMap
	for _, node := range nodeNames {
		items, err := dc.list(ctx, gvrNMState, "")
		if err != nil {
			continue
		}
		for _, item := range items {
			if !strings.EqualFold(item.GetName(), node) && !strings.Contains(item.GetName(), node) {
				continue
			}
			ifaces, _, _ := unstructured.NestedSlice(item.Object, "status", "interfaces")
			reports = append(reports, NMStateReport{
				NodeName: node, ReportName: item.GetName(), Interfaces: len(ifaces),
			})
			for _, raw := range ifaces {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				name := stringField(m, "name")
				if name == "" {
					continue
				}
				ifmap := InterfaceMap{
					NodeName:     node,
					PhysicalName: name,
					Type:         stringField(m, "type"),
					MAC:          stringField(m, "mac-address"),
					State:        stringField(m, "state"),
				}
				if strings.Contains(name, "bond") {
					ifmap.Type = "bond"
				}
				if strings.HasPrefix(name, "br-") {
					ifmap.Bridge = name
				}
				for _, br := range infra.OVSBridges {
					if br.NodeName == node && br.Name == "br-ext" {
						ifmap.Bridge = "br-ext"
						ifmap.LogicalPort = "physnet"
					}
				}
				ifmaps = append(ifmaps, ifmap)
			}
		}
	}
	return reports, ifmaps
}

func synthesizeHostRoutes(nodeNames []string, resp *trace.DiscoverResponse) []HostRoute {
	var out []HostRoute
	dest := ""
	if resp != nil && resp.Destination.Mode == "ip" {
		dest = strings.TrimSpace(resp.Destination.IP)
	}
	for _, node := range nodeNames {
		if dest != "" && !strings.EqualFold(dest, "external") {
			out = append(out, HostRoute{
				NodeName: node, Table: "main", DestCIDR: dest + "/32", Dev: "br-ext", Scope: "global",
			})
		}
		out = append(out, HostRoute{
			NodeName: node, Table: "254", DestCIDR: "0.0.0.0/0", Gateway: "169.254.0.1", Dev: "ovn-k8s-mp0",
		})
	}
	return out
}

func physicalTopology(physical PhysicalPlaneResult, nodeNames []string) ([]TopologyNode, []TopologyEdge) {
	nodes := make([]TopologyNode, 0, len(nodeNames)+len(physical.InterfaceMaps))
	edges := make([]TopologyEdge, 0)
	for _, node := range nodeNames {
		id := fmt.Sprintf("node:%s", node)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: node, Neo4jLabel: "Node", Layer: LayerPhysical,
			Shape: ShapeRectangle, Kind: "node", Sensitive: true,
		})
	}
	for _, im := range physical.InterfaceMaps {
		id := fmt.Sprintf("iface:%s:%s", im.NodeName, im.PhysicalName)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: im.PhysicalName, Neo4jLabel: "Interface", Layer: LayerPhysical,
			Shape: ShapeRectangle, Kind: im.Type, Detail: im.MAC, Sensitive: true,
			Properties: map[string]string{"bridge": im.Bridge},
		})
		nodeID := fmt.Sprintf("node:%s", im.NodeName)
		edges = append(edges, TopologyEdge{
			ID: fmt.Sprintf("bind:%s:%s", im.NodeName, im.PhysicalName),
			From: nodeID, To: id, RelType: "BINDS_TO", Layer: LayerPhysical, State: EdgeTheoryOnly,
		})
	}
	for _, r := range physical.NMStateReports {
		id := fmt.Sprintf("nmstate:%s", r.NodeName)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: "NMState", Neo4jLabel: "NMStateConfig", Layer: LayerPhysical,
			Shape: ShapeRectangle, Kind: "nmstate", Detail: fmt.Sprintf("%d interfaces", r.Interfaces),
			Sensitive: true,
		})
		nodeID := fmt.Sprintf("node:%s", r.NodeName)
		edges = append(edges, TopologyEdge{
			ID: fmt.Sprintf("cfg:%s", r.NodeName), From: nodeID, To: id,
			RelType: "CONFIGURED_BY", Layer: LayerPhysical, State: EdgeTheoryOnly,
		})
	}
	return nodes, edges
}

// NodeNamesFromPods helper for external callers.
func NodeNamesFromPods(pods ...[]spcgk8s.PodDetail) []string {
	resp := &traceDiscoverPods{source: pods}
	return collectNodeNames(resp.asResponse())
}

type traceDiscoverPods struct {
	source [][]spcgk8s.PodDetail
}

func (t *traceDiscoverPods) asResponse() *trace.DiscoverResponse {
	if t == nil {
		return nil
	}
	var out trace.DiscoverResponse
	for _, group := range t.source {
		out.SourcePods = append(out.SourcePods, group...)
	}
	return &out
}
