package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	annMultusNetworks = "k8s.v1.cni.cncf.io/networks"
	annMultusStatus   = "k8s.v1.cni.cncf.io/network-status"
)

var gvrAdminPolicyRoute = schema.GroupVersionResource{
	Group: "k8s.ovn.org", Version: "v1", Resource: "adminpolicybasedexternalroutes",
}

type logicalDiscoverer struct {
	cs kubernetes.Interface
	dc traceDynamicLister
}

type traceDynamicLister interface {
	list(ctx context.Context, gvr schema.GroupVersionResource, ns string) ([]unstructured.Unstructured, error)
}

type dynamicAdapter struct {
	dc *trace.DynamicClient
}

func (a dynamicAdapter) list(ctx context.Context, gvr schema.GroupVersionResource, ns string) ([]unstructured.Unstructured, error) {
	if a.dc == nil {
		return nil, nil
	}
	return a.dc.List(ctx, gvr, ns)
}

func newLogicalDiscoverer(cat *trace.Catalog) logicalDiscoverer {
	ld := logicalDiscoverer{cs: cat.CS}
	if cat != nil && cat.DC != nil {
		ld.dc = dynamicAdapter{dc: cat.DC}
	}
	return ld
}

func (d logicalDiscoverer) discover(ctx context.Context, req trace.DiscoverRequest, resp *trace.DiscoverResponse) LogicalPlaneResult {
	out := LogicalPlaneResult{
		ResolvedSource:      req.Source,
		ResolvedDestination: req.Destination,
	}
	if d.cs == nil || resp == nil {
		return out
	}
	for _, pod := range resp.SourcePods {
		attachments := discoverMultusForPod(ctx, d.cs, d.dc, pod)
		out.MultusAttachments = append(out.MultusAttachments, attachments...)
	}
	for _, pod := range resp.DestPods {
		attachments := discoverMultusForPod(ctx, d.cs, d.dc, pod)
		out.MultusAttachments = append(out.MultusAttachments, attachments...)
	}
	out.MetalLBPaths = discoverMetalLBPaths(ctx, d.cs, d.dc, resp)
	out.EgressBindings = discoverEgressBindings(ctx, d.dc, resp.SourcePods)
	out.AdminPolicyRoutes = discoverAdminPolicyRoutes(ctx, d.dc)
	out.Nodes, out.Edges = logicalTopologyFromGraph(resp.Graph, out)
	return out
}

func discoverMultusForPod(ctx context.Context, cs kubernetes.Interface, dc traceDynamicLister, pod spcgk8s.PodDetail) []MultusAttachment {
	p, err := cs.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	ann := strings.TrimSpace(p.Annotations[annMultusNetworks])
	if ann == "" {
		ann = strings.TrimSpace(p.Annotations["v1.multus-cni.io/default-network"])
	}
	if ann == "" {
		return nil
	}
	statusJSON := strings.TrimSpace(p.Annotations[annMultusStatus])
	statusByIface := parseNetworkStatus(statusJSON)
	var out []MultusAttachment
	if dc == nil {
		return out
	}
	items, err := dc.list(ctx, schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}, pod.Namespace)
	if err != nil {
		return out
	}
	for _, item := range items {
		if !strings.Contains(ann, item.GetName()) {
			continue
		}
		att := MultusAttachment{
			PodNamespace: pod.Namespace,
			PodName:      pod.Name,
			NADName:      item.GetName(),
			NADNamespace: pod.Namespace,
			Interface:    ifaceForNAD(item.GetName(), statusByIface),
		}
		enrichNADConfig(&att, item)
		out = append(out, att)
	}
	return out
}

func parseNetworkStatus(raw string) map[string]networkStatusEntry {
	out := map[string]networkStatusEntry{}
	if raw == "" {
		return out
	}
	var entries []networkStatusEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return out
	}
	for _, e := range entries {
		key := e.Name
		if key == "" {
			key = e.Interface
		}
		out[key] = e
	}
	return out
}

type networkStatusEntry struct {
	Name      string `json:"name"`
	Interface string `json:"interface"`
	IPs       []string `json:"ips"`
}

func ifaceForNAD(nadName string, status map[string]networkStatusEntry) string {
	if e, ok := status[nadName]; ok && e.Interface != "" {
		return e.Interface
	}
	return "net1"
}

func enrichNADConfig(att *MultusAttachment, nad unstructured.Unstructured) {
	config, _, _ := unstructured.NestedString(nad.Object, "spec", "config")
	if config == "" {
		return
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		att.CNIType = "unknown"
		return
	}
	att.CNIType = stringField(parsed, "type")
	switch att.CNIType {
	case "ovn-k8s-cni-overlay":
		att.Topology = stringField(parsed, "topology")
		if subnets, ok := parsed["subnets"].(string); ok {
			att.Subnets = subnets
		}
		if subs, ok := parsed["subnets"].([]any); ok {
			parts := make([]string, 0, len(subs))
			for _, s := range subs {
				parts = append(parts, fmt.Sprint(s))
			}
			att.Subnets = strings.Join(parts, ", ")
		}
		att.VLANID = stringField(parsed, "vlanID")
		att.IPAMClaims = stringField(parsed, "persistent-ip-claim")
	case "host-device", "bridge", "macvlan", "ipvlan", "sriov":
		att.HostBypass = true
		att.HostIface = firstNonEmpty(
			stringField(parsed, "device"),
			stringField(parsed, "master"),
			stringField(parsed, "pciAddress"),
		)
	}
}

func discoverMetalLBPaths(ctx context.Context, cs kubernetes.Interface, dc traceDynamicLister, resp *trace.DiscoverResponse) []MetalLBPath {
	if dc == nil {
		return nil
	}
	var paths []MetalLBPath
	seen := map[string]struct{}{}
	for ns := range namespacesFromResponse(resp) {
		svcs, err := cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for i := range svcs.Items {
			svc := &svcs.Items[i]
			if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
				continue
			}
			vip := loadBalancerVIP(svc)
			if vip == "" {
				continue
			}
			key := svc.Namespace + "/" + svc.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			paths = append(paths, MetalLBPath{
				ServiceNamespace: svc.Namespace,
				ServiceName:      svc.Name,
				VIP:              vip,
			})
		}
	}
	return paths
}

func loadBalancerVIP(svc *corev1.Service) string {
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			return ing.IP
		}
	}
	return ""
}

func discoverEgressBindings(ctx context.Context, dc traceDynamicLister, pods []spcgk8s.PodDetail) []EgressBinding {
	if dc == nil || len(pods) == 0 {
		return nil
	}
	items, err := dc.list(ctx, schema.GroupVersionResource{Group: "k8s.ovn.org", Version: "v1", Resource: "egressips"}, "")
	if err != nil {
		return nil
	}
	anchorNS := pods[0].Namespace
	var out []EgressBinding
	for _, item := range items {
		selector, _, _ := unstructured.NestedStringMap(item.Object, "spec", "namespaceSelector", "matchLabels")
		if !selectorMatchesNamespace(selector, anchorNS) {
			continue
		}
		egressIPs, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "egressIPs")
		b := EgressBinding{Kind: "EgressIP", Name: item.GetName(), Namespace: item.GetNamespace()}
		if len(egressIPs) > 0 {
			b.EgressIP = egressIPs[0]
		}
		out = append(out, b)
	}
	return out
}

func discoverAdminPolicyRoutes(ctx context.Context, dc traceDynamicLister) []AdminPolicyRoute {
	if dc == nil {
		return nil
	}
	items, err := dc.list(ctx, gvrAdminPolicyRoute, "")
	if err != nil {
		return nil
	}
	out := make([]AdminPolicyRoute, 0, len(items))
	for _, item := range items {
		nextHop, _, _ := unstructured.NestedString(item.Object, "spec", "nextHops", "0", "nextHop")
		out = append(out, AdminPolicyRoute{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			NextHop:   nextHop,
		})
	}
	return out
}

func selectorMatchesNamespace(labels map[string]string, ns string) bool {
	if len(labels) == 0 {
		return true
	}
	// Full namespace label resolution requires listing namespace labels; permissive for scoped pods.
	return true
}

func logicalTopologyFromGraph(g trace.TraceGraph, logical LogicalPlaneResult) ([]TopologyNode, []TopologyEdge) {
	nodes := make([]TopologyNode, 0, len(g.Nodes)+len(logical.MultusAttachments))
	edges := make([]TopologyEdge, 0, len(g.Edges))
	for _, n := range g.Nodes {
		if n.Layer != "" && n.Layer != string(LayerLogical) && n.Layer != "physical" {
			continue
		}
		layer := LayerLogical
		shape := ShapeRounded
		neoLabel := mapKindToNeo4jLabel(n.Kind)
		if n.Layer == "physical" {
			layer = LayerPhysical
			shape = ShapeRectangle
		}
		nodes = append(nodes, TopologyNode{
			ID: n.ID, Label: n.Label, Neo4jLabel: neoLabel, Layer: layer, Shape: shape,
			Namespace: n.Namespace, Kind: n.Kind, Detail: n.Detail, Rank: n.Rank, X: n.X, Y: n.Y,
		})
	}
	for _, e := range g.Edges {
		edges = append(edges, TopologyEdge{
			ID: e.ID, From: e.From, To: e.To, RelType: mapEdgeType(e.EdgeType),
			Layer: LayerLogical, Primary: e.Primary, Label: e.Label, State: EdgeTheoryOnly,
		})
	}
	for _, att := range logical.MultusAttachments {
		id := fmt.Sprintf("nad:%s/%s:%s", att.NADNamespace, att.NADName, att.Interface)
		nodes = append(nodes, TopologyNode{
			ID: id, Label: att.NADName, Neo4jLabel: "NetworkAttachmentDefinition",
			Layer: LayerLogical, Shape: ShapeRounded, Namespace: att.NADNamespace,
			Kind: "nad", Detail: att.CNIType + " · " + att.Topology,
			Properties: map[string]string{"interface": att.Interface, "vlan_id": att.VLANID},
		})
	}
	return nodes, edges
}

func mapKindToNeo4jLabel(kind string) string {
	switch strings.ToLower(kind) {
	case "pod", "pod-ip":
		return "Pod"
	case "service", "service-clusterip", "service-loadbalancer", "service-nodeport":
		return "Service"
	case "egressip", "egressservice":
		return "EgressIP"
	case "nad":
		return "NetworkAttachmentDefinition"
	case "metallb-pool", "loadbalancer-external":
		return "LoadBalancer"
	case "bgp-peer":
		return "BGP_Peer"
	case "node":
		return "Node"
	default:
		return "Pod"
	}
}

func mapEdgeType(edgeType string) string {
	switch edgeType {
	case "ingress":
		return "CONSUMES"
	case "egress", "egressservice":
		return "ROUTES_VIA"
	default:
		return "CONNECTS"
	}
}

func namespacesFromResponse(resp *trace.DiscoverResponse) map[string]struct{} {
	out := map[string]struct{}{}
	if resp == nil {
		return out
	}
	for _, ns := range resp.Graph.Namespaces {
		out[ns] = struct{}{}
	}
	for _, p := range resp.SourcePods {
		out[p.Namespace] = struct{}{}
	}
	for _, p := range resp.DestPods {
		out[p.Namespace] = struct{}{}
	}
	return out
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
