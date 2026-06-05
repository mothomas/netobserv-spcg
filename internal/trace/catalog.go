package trace

import (
	"context"
	"fmt"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	gvrRoute = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
	gvrEgressIP = schema.GroupVersionResource{Group: "k8s.ovn.org", Version: "v1", Resource: "egressips"}
	gvrEgressService = schema.GroupVersionResource{Group: "egressservice.k8s.ovn.org", Version: "v1", Resource: "egressservices"}
	gvrEgressNetwork = schema.GroupVersionResource{Group: "network.openshift.io", Version: "v1", Resource: "egressnetworks"}
	gvrBGPPeer = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "bgppeers"}
	gvrIPPool = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "ipaddresspools"}
	gvrNAD = schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
)

// Catalog discovers ingress/egress infrastructure paths for a target pod.
type Catalog struct {
	CS kubernetes.Interface
	DC *DynamicClient
}

// Resolve expands selections, discovers paths, and builds the infrastructure graph.
func (c *Catalog) Resolve(ctx context.Context, req DiscoverRequest) (*DiscoverResponse, error) {
	if c == nil || c.CS == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	if len(req.Selections) == 0 {
		return nil, fmt.Errorf("at least one workload selection is required")
	}
	resolved, err := spcgk8s.ResolveCaptureSelections(ctx, c.CS, req.Selections)
	if err != nil {
		return nil, err
	}
	target, err := pickTargetPod(resolved.Pods)
	if err != nil {
		return nil, err
	}

	nsScope := namespaceScope(req.Namespaces, target.Namespace)
	builder := newGraphBuilder(target, nsScope)

	builder.addNode(nodeID("pod", target.Namespace, target.Name), target.Name, "pod", target.Namespace, true, target.PodIP)
	if target.NodeName != "" {
		builder.addNode(nodeID("node", "", target.NodeName), target.NodeName, "node", "", false, "scheduled node")
		builder.addEdge(builder.podID, builder.nodeID, "scheduled", true, "")
	}

	svcHits, err := c.discoverServices(ctx, target, nsScope, builder)
	if err != nil {
		return nil, err
	}
	if err := c.discoverRoutes(ctx, target.Namespace, svcHits, builder); err != nil {
		return nil, err
	}
	if err := c.discoverMetalLB(ctx, svcHits, builder); err != nil {
		return nil, err
	}
	if err := c.discoverEgress(ctx, nsScope, target, builder); err != nil {
		return nil, err
	}
	if err := c.discoverNADs(ctx, target, builder); err != nil {
		return nil, err
	}
	_ = c.discoverNetworkPolicies(ctx, target, builder)

	graph := builder.finish(req.TraceID)
	return &DiscoverResponse{
		TraceID:   req.TraceID,
		TargetPod: target,
		Graph:     graph,
		Resolved:  *resolved,
	}, nil
}

func pickTargetPod(pods []spcgk8s.PodDetail) (spcgk8s.PodDetail, error) {
	if len(pods) == 0 {
		return spcgk8s.PodDetail{}, fmt.Errorf("no pods resolved from selections")
	}
	if len(pods) == 1 {
		return pods[0], nil
	}
	return spcgk8s.PodDetail{}, fmt.Errorf("packet trace requires exactly one pod target (got %d); select a single pod or one-replica owner", len(pods))
}

func namespaceScope(requested []string, podNS string) map[string]struct{} {
	out := map[string]struct{}{podNS: {}}
	for _, ns := range requested {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			out[ns] = struct{}{}
		}
	}
	return out
}

func (c *Catalog) discoverServices(ctx context.Context, pod spcgk8s.PodDetail, scope map[string]struct{}, b *graphBuilder) (map[string]corev1.Service, error) {
	hits := map[string]corev1.Service{}
	podIPs := podIPSet(pod)
	for ns := range scope {
		svcs, err := c.CS.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list services %s: %w", ns, err)
		}
		for i := range svcs.Items {
			svc := svcs.Items[i]
			if !serviceTargetsPod(ctx, c.CS, &svc, podIPs) {
				continue
			}
			key := svc.Namespace + "/" + svc.Name
			hits[key] = svc
			kind := serviceKind(&svc)
			id := nodeID("service", svc.Namespace, svc.Name)
			detail := string(svc.Spec.ClusterIP)
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				for _, ing := range svc.Status.LoadBalancer.Ingress {
					if ing.IP != "" {
						detail = ing.IP
						break
					}
					if ing.Hostname != "" {
						detail = ing.Hostname
						break
					}
				}
			}
			if svc.Spec.Type == corev1.ServiceTypeNodePort && len(svc.Spec.Ports) > 0 && svc.Spec.Ports[0].NodePort != 0 {
				detail = fmt.Sprintf("NodePort %d", svc.Spec.Ports[0].NodePort)
			}
			b.addNode(id, svc.Name, kind, svc.Namespace, false, detail)
			b.addPath("ingress", svc.Name, svc.Namespace, kind, "discovered", detail)
			b.addEdge(id, b.podID, "direct", true, "")
		}
	}
	return hits, nil
}

func serviceKind(svc *corev1.Service) string {
	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		return "service-loadbalancer"
	case corev1.ServiceTypeNodePort:
		return "service-nodeport"
	default:
		return "service-clusterip"
	}
}

func podIPSet(pod spcgk8s.PodDetail) map[string]struct{} {
	out := map[string]struct{}{}
	if ip := strings.TrimSpace(pod.PodIP); ip != "" {
		out[ip] = struct{}{}
	}
	for _, ip := range pod.PodIPs {
		if t := strings.TrimSpace(ip); t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}

func serviceTargetsPod(ctx context.Context, cs kubernetes.Interface, svc *corev1.Service, podIPs map[string]struct{}) bool {
	if len(podIPs) == 0 {
		return false
	}
	slices, err := cs.DiscoveryV1().EndpointSlices(svc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + svc.Name,
	})
	if err != nil {
		return false
	}
	for _, sl := range slices.Items {
		for _, ep := range sl.Endpoints {
			for _, addr := range ep.Addresses {
				if _, ok := podIPs[strings.TrimSpace(addr)]; ok {
					return true
				}
			}
		}
	}
	return false
}

func (c *Catalog) discoverRoutes(ctx context.Context, podNS string, services map[string]corev1.Service, b *graphBuilder) error {
	if c.DC == nil {
		return nil
	}
	items, err := c.DC.list(ctx, gvrRoute, podNS)
	if err != nil {
		return nil // routes API not available
	}
	svcByName := map[string]corev1.Service{}
	for _, svc := range services {
		svcByName[svc.Name] = svc
	}
	for _, item := range items {
		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		toName, _, _ := unstructured.NestedString(item.Object, "spec", "to", "name")
		if toName == "" || host == "" {
			continue
		}
		if _, ok := svcByName[toName]; !ok {
			continue
		}
		id := nodeID("route", item.GetNamespace(), item.GetName())
		b.addNode(id, item.GetName(), "route", item.GetNamespace(), false, host)
		b.addPath("ingress", item.GetName(), item.GetNamespace(), "route", "discovered", "https://"+host)
		svcID := nodeID("service", item.GetNamespace(), toName)
		b.addEdge(id, svcID, "https", true, host)

		extID := nodeID("external", "", "client-route")
		if !b.hasNode(extID) {
			b.addNode(extID, "Client", "external-client", "", false, "via Route")
		}
		b.addEdge(extID, id, "ingress", true, "")
	}
	return nil
}

func (c *Catalog) discoverMetalLB(ctx context.Context, services map[string]corev1.Service, b *graphBuilder) error {
	if c.DC == nil {
		return nil
	}
	for _, svc := range services {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		vip := ""
		for _, ing := range svc.Status.LoadBalancer.Ingress {
			if ing.IP != "" {
				vip = ing.IP
				break
			}
		}
		if vip == "" {
			continue
		}
		lbID := nodeID("loadbalancer", svc.Namespace, svc.Name+"-vip")
		b.addNode(lbID, vip, "loadbalancer-external", svc.Namespace, false, "MetalLB VIP")
		b.addPath("ingress", vip, svc.Namespace, "metallb-pool", "discovered", svc.Name)
		svcID := nodeID("service", svc.Namespace, svc.Name)
		b.addEdge(lbID, svcID, "direct", false, "")

		extID := nodeID("external", "", "client-lb")
		if !b.hasNode(extID) {
			b.addNode(extID, "Client", "external-client", "", false, "via LB")
		}
		b.addEdge(extID, lbID, "ingress", false, "")
	}

	peers, err := c.DC.list(ctx, gvrBGPPeer, "metallb-system")
	if err != nil {
		peers, _ = c.DC.list(ctx, gvrBGPPeer, "")
	}
	for _, p := range peers {
		id := nodeID("bgp-peer", p.GetNamespace(), p.GetName())
		addr, _, _ := unstructured.NestedString(p.Object, "spec", "peerAddress")
		b.addNode(id, p.GetName(), "bgp-peer", p.GetNamespace(), false, addr)
		b.addPath("ingress", p.GetName(), p.GetNamespace(), "bgp-peer", "discovered", addr)
	}
	_, _ = c.DC.list(ctx, gvrIPPool, "metallb-system")
	return nil
}

func (c *Catalog) discoverEgress(ctx context.Context, scope map[string]struct{}, pod spcgk8s.PodDetail, b *graphBuilder) error {
	if c.DC == nil {
		return nil
	}
	egressIPs, _ := c.DC.list(ctx, gvrEgressIP, "")
	for _, item := range egressIPs {
		ns, _, _ := unstructured.NestedString(item.Object, "spec", "namespace")
		if ns != "" {
			if _, ok := scope[ns]; !ok {
				continue
			}
		}
		id := nodeID("egressip", "", item.GetName())
		statusIP, _, _ := unstructured.NestedString(item.Object, "status", "items", "0", "egressIP")
		b.addNode(id, item.GetName(), "egressip", ns, false, statusIP)
		b.addPath("egress", item.GetName(), ns, "egressip", scopeStatus(ns, scope), statusIP)
		b.addEdge(b.podID, id, "egress", true, "")
		if statusIP != "" {
			dstID := nodeID("external", "", "dest-egress")
			if !b.hasNode(dstID) {
				b.addNode(dstID, "Dest", "external", "", false, "egress path")
			}
			b.addEdge(id, dstID, "egress", true, "")
		} else if b.nodeID != "" {
			bondID := nodeID("bond", "", b.target.NodeName+"-bond0")
			if b.hasNode(bondID) {
				b.addEdge(bondID, id, "egress", true, "", true)
			}
		}
	}

	for ns := range scope {
		ess, _ := c.DC.list(ctx, gvrEgressService, ns)
		for _, item := range ess {
			id := nodeID("egressservice", ns, item.GetName())
			ip, _, _ := unstructured.NestedString(item.Object, "status", "assignedIP")
			b.addNode(id, item.GetName(), "egressservice", ns, false, ip)
			b.addPath("egress", item.GetName(), ns, "egressservice", "discovered", ip)
			if ns == pod.Namespace {
				b.addEdge(b.podID, id, "egressservice", false, "")
			} else {
				b.addPath("egress", item.GetName(), ns, "egressservice", "out_of_scope", ip)
			}
		}
	}

	egressNets, _ := c.DC.list(ctx, gvrEgressNetwork, "")
	for _, item := range egressNets {
		id := nodeID("egress-router", "", item.GetName())
		b.addNode(id, item.GetName(), "egress-router", "", false, "EgressNetwork")
		b.addPath("egress", item.GetName(), "", "egress-router", "discovered", "OpenShift egress router")
	}

	if pod.NodeName != "" {
		ovnID := nodeID("ovn", pod.Namespace, pod.Name)
		b.addNode(ovnID, "OVN port", "ovn-logical-port", pod.Namespace, false, "lp_"+pod.Name)
		b.addEdge(b.podID, ovnID, "direct", true, "")
		if b.nodeID != "" {
			b.addEdge(ovnID, b.nodeID, "direct", true, "")
		}
		bondID := nodeID("bond", "", pod.NodeName+"-bond0")
		b.addNode(bondID, "bond0", "bond", "", false, "eth0+eth1")
		b.addPath("host", "bond0", "", "bond", "discovered", pod.NodeName)
		if b.nodeID != "" {
			b.addEdge(b.nodeID, bondID, "host", true, "")
		}
	}
	return nil
}

func scopeStatus(ns string, scope map[string]struct{}) string {
	if ns == "" {
		return "discovered"
	}
	if _, ok := scope[ns]; ok {
		return "discovered"
	}
	return "out_of_scope"
}

func (c *Catalog) discoverNADs(ctx context.Context, pod spcgk8s.PodDetail, b *graphBuilder) error {
	if c.DC == nil {
		return nil
	}
	items, err := c.DC.list(ctx, gvrNAD, pod.Namespace)
	if err != nil {
		return nil
	}
	ann, err := podMultusAnnotation(ctx, c.CS, pod)
	if err != nil || ann == "" {
		return nil
	}
	for _, item := range items {
		if !strings.Contains(ann, item.GetName()) {
			continue
		}
		id := nodeID("nad", pod.Namespace, item.GetName())
		config, _, _ := unstructured.NestedString(item.Object, "spec", "config")
		b.addNode(id, item.GetName(), "nad", pod.Namespace, false, truncate(config, 48))
		b.addPath("host", item.GetName(), pod.Namespace, "nad", "discovered", "secondary interface")
		b.addEdge(b.podID, id, "direct", false, "")
	}
	return nil
}

func podMultusAnnotation(ctx context.Context, cs kubernetes.Interface, pod spcgk8s.PodDetail) (string, error) {
	p, err := cs.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if ann := strings.TrimSpace(p.Annotations["k8s.v1.cni.cncf.io/networks"]); ann != "" {
		return ann, nil
	}
	return strings.TrimSpace(p.Annotations["v1.multus-cni.io/default-network"]), nil
}

func (c *Catalog) discoverNetworkPolicies(ctx context.Context, pod spcgk8s.PodDetail, b *graphBuilder) error {
	policies, err := c.CS.NetworkingV1().NetworkPolicies(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, np := range policies.Items {
		id := nodeID("netpol", pod.Namespace, np.Name)
		b.addNode(id, np.Name, "networkpolicy", pod.Namespace, false, "policy")
		b.addPath("host", np.Name, pod.Namespace, "networkpolicy", "discovered", "")
		b.addEdge(id, b.podID, "policy-deny", false, "")
	}
	return nil
}

// OpenCatalog builds a Catalog from standard and dynamic clients.
func OpenCatalog(cs kubernetes.Interface, cfg *rest.Config) (*Catalog, error) {
	cat := &Catalog{CS: cs}
	if cfg != nil {
		dc, err := NewDynamicClient(cfg)
		if err == nil {
			cat.DC = dc
		}
	}
	return cat, nil
}
