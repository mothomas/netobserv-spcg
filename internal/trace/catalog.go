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
	gvrRoute         = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
	gvrEgressIP      = schema.GroupVersionResource{Group: "k8s.ovn.org", Version: "v1", Resource: "egressips"}
	gvrEgressService = schema.GroupVersionResource{Group: "egressservice.k8s.ovn.org", Version: "v1", Resource: "egressservices"}
	gvrEgressNetwork = schema.GroupVersionResource{Group: "network.openshift.io", Version: "v1", Resource: "egressnetworks"}
	gvrBGPPeer       = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "bgppeers"}
	gvrIPPool        = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "ipaddresspools"}
	gvrNAD           = schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
)

// Catalog discovers ingress/egress infrastructure paths between source and destination endpoints.
type Catalog struct {
	CS kubernetes.Interface
	DC *DynamicClient
}

// Resolve expands endpoints, discovers paths, and builds the infrastructure graph.
func (c *Catalog) Resolve(ctx context.Context, req DiscoverRequest) (*DiscoverResponse, error) {
	if c == nil || c.CS == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	if err := normalizeDiscoverRequest(&req); err != nil {
		return nil, err
	}
	if len(req.Namespaces) == 0 {
		return nil, fmt.Errorf("namespaces are required")
	}

	src, err := resolveEndpoint(ctx, c.CS, req.Source, req.Namespaces)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	dst, err := resolveEndpoint(ctx, c.CS, req.Destination, req.Namespaces)
	if err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	if len(src.Pods) == 0 && src.IPNode == nil {
		return nil, fmt.Errorf("source resolved to no targets")
	}

	scope := namespacesForScope(req.Namespaces, src.Pods, dst.Pods)
	builder := newGraphBuilder(src.Pods, dst.Pods, dst.IPNode, scope)
	builder.seedEndpoints()

	allSvcHits := map[string]corev1.Service{}
	for _, pod := range src.Pods {
		hits, err := c.discoverServices(ctx, pod, scope, builder)
		if err != nil {
			return nil, err
		}
		for k, v := range hits {
			allSvcHits[k] = v
		}
		_ = c.discoverNADs(ctx, pod, builder)
	}
	for _, pod := range dst.Pods {
		hits, err := c.discoverServices(ctx, pod, scope, builder)
		if err != nil {
			return nil, err
		}
		for k, v := range hits {
			allSvcHits[k] = v
		}
	}
	if len(src.Pods) > 0 {
		if err := c.discoverRoutes(ctx, src.Pods[0].Namespace, allSvcHits, builder); err != nil {
			return nil, err
		}
	}
	if err := c.discoverMetalLB(ctx, allSvcHits, builder); err != nil {
		return nil, err
	}
	if len(src.Pods) > 0 {
		if err := c.discoverEgress(ctx, scope, src.Pods[0], builder); err != nil {
			return nil, err
		}
	}

	graph := builder.finish(req.TraceID)
	target := src.Pods[0]
	resolved, _ := spcgk8s.ResolveCaptureSelections(ctx, c.CS, selectionsFromPods(src.Pods))

	return &DiscoverResponse{
		TraceID:     req.TraceID,
		Source:      req.Source,
		Destination: req.Destination,
		SourcePods:  src.Pods,
		DestPods:    dst.Pods,
		TargetPod:   target,
		Graph:       graph,
		Resolved:    derefResolved(resolved),
	}, nil
}

func selectionsFromPods(pods []spcgk8s.PodDetail) []spcgk8s.CaptureSelection {
	out := make([]spcgk8s.CaptureSelection, 0, len(pods))
	for _, p := range pods {
		out = append(out, spcgk8s.CaptureSelection{
			Namespace: p.Namespace,
			Type:      "pod",
			PodName:   p.Name,
			PodUID:    p.UID,
		})
	}
	return out
}

func derefResolved(r *spcgk8s.ResolvedCapture) spcgk8s.ResolvedCapture {
	if r == nil {
		return spcgk8s.ResolvedCapture{}
	}
	return *r
}

func (c *Catalog) discoverServices(ctx context.Context, pod spcgk8s.PodDetail, scope map[string]struct{}, b *graphBuilder) (map[string]corev1.Service, error) {
	hits := map[string]corev1.Service{}
	podIPs := podIPSet(pod)
	podID := podNodeID(pod)
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
			b.addNode(id, svc.Name, kind, svc.Namespace, false, false, rankForKind(kind), detail)
			b.addPath("ingress", svc.Name, svc.Namespace, kind, "discovered", detail)
			if _, src := b.sourceIDs[podID]; src {
				b.addEdge(id, podID, "direct", false, "")
			} else if _, dst := b.destIDs[podID]; dst {
				b.addEdge(id, podID, "direct", false, "")
			}
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
		return nil
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
		b.addNode(id, item.GetName(), "route", item.GetNamespace(), false, false, rankForKind("route"), host)
		b.addPath("ingress", item.GetName(), item.GetNamespace(), "route", "discovered", "https://"+host)
		svcID := nodeID("service", item.GetNamespace(), toName)
		b.addEdge(id, svcID, "https", false, host)

		extID := nodeID("external", "", "client-route")
		if !b.hasNode(extID) {
			b.addNode(extID, "Client", "external-client", "", false, false, rankForKind("external-client"), "via Route")
		}
		b.addEdge(extID, id, "ingress", false, "")
	}
	return nil
}

func (c *Catalog) discoverEgress(ctx context.Context, scope map[string]struct{}, pod spcgk8s.PodDetail, b *graphBuilder) error {
	if c.DC == nil {
		return nil
	}
	podID := podNodeID(pod)
	destID := ""
	for id := range b.destIDs {
		destID = id
		break
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
		b.addNode(id, item.GetName(), "egressip", ns, false, false, rankForKind("egressip"), statusIP)
		b.addPath("egress", item.GetName(), ns, "egressip", scopeStatus(ns, scope), statusIP)
		b.addEdge(podID, id, "egress", false, "")
		if destID != "" {
			b.addEdge(id, destID, "egress", false, "")
		} else if statusIP != "" {
			dstID := nodeID("external", "", "dest-egress")
			if !b.hasNode(dstID) {
				b.addNode(dstID, "Dest", "external", "", false, false, rankDest, "egress path")
			}
			b.addEdge(id, dstID, "egress", false, "")
		}
	}

	for ns := range scope {
		ess, _ := c.DC.list(ctx, gvrEgressService, ns)
		for _, item := range ess {
			id := nodeID("egressservice", ns, item.GetName())
			ip, _, _ := unstructured.NestedString(item.Object, "status", "assignedIP")
			b.addNode(id, item.GetName(), "egressservice", ns, false, false, rankForKind("egressservice"), ip)
			b.addPath("egress", item.GetName(), ns, "egressservice", "discovered", ip)
			if ns == pod.Namespace {
				b.addEdge(podID, id, "egressservice", false, "")
			}
		}
	}

	if pod.NodeName != "" {
		ovnID := nodeID("ovn", pod.Namespace, pod.Name)
		b.addNode(ovnID, "OVN port", "ovn-logical-port", pod.Namespace, false, false, rankForKind("ovn-logical-port"), "lp_"+pod.Name)
		b.addEdge(podID, ovnID, "direct", false, "")
		nodeIDVal := nodeID("node", "", pod.NodeName)
		if b.hasNode(nodeIDVal) {
			b.addEdge(ovnID, nodeIDVal, "direct", false, "")
		}
		bondID := nodeID("bond", "", pod.NodeName+"-bond0")
		b.addNode(bondID, "bond0", "bond", "", false, false, rankForKind("bond"), "eth0+eth1")
		b.addPath("host", "bond0", "", "bond", "discovered", pod.NodeName)
		if b.hasNode(nodeIDVal) {
			b.addEdge(nodeIDVal, bondID, "host", false, "")
		}
		if destID != "" {
			b.addEdge(bondID, destID, "egress", false, "")
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
	podID := podNodeID(pod)
	for _, item := range items {
		if !strings.Contains(ann, item.GetName()) {
			continue
		}
		id := nodeID("nad", pod.Namespace, item.GetName())
		config, _, _ := unstructured.NestedString(item.Object, "spec", "config")
		b.addNode(id, item.GetName(), "nad", pod.Namespace, false, false, rankForKind("nad"), truncate(config, 48))
		b.addPath("host", item.GetName(), pod.Namespace, "nad", "discovered", "secondary interface")
		b.addEdge(podID, id, "direct", false, "")
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
	podID := podNodeID(pod)
	for _, np := range policies.Items {
		id := nodeID("netpol", pod.Namespace, np.Name)
		b.addNode(id, np.Name, "networkpolicy", pod.Namespace, false, false, rankForKind("networkpolicy"), "policy")
		b.addPath("host", np.Name, pod.Namespace, "networkpolicy", "discovered", "")
		b.addEdge(id, podID, "policy-deny", false, "")
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
