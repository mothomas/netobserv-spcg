package trace

import (
	"context"
	"fmt"
	"net"
	"strings"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EndpointMode selects IP or namespace/workload targeting.
type EndpointMode string

const (
	EndpointIP        EndpointMode = "ip"
	EndpointNamespace EndpointMode = "namespace"
)

// TraceEndpoint is a source or destination trace target.
type TraceEndpoint struct {
	Mode          EndpointMode `json:"mode"`
	IP            string       `json:"ip,omitempty"`
	External      bool         `json:"external,omitempty"`
	Namespace     string       `json:"namespace,omitempty"`
	Type          string       `json:"type,omitempty"` // pod | owner
	PodName       string       `json:"pod_name,omitempty"`
	PodUID        string       `json:"pod_uid,omitempty"`
	OwnerKind     string       `json:"owner_kind,omitempty"`
	OwnerName     string       `json:"owner_name,omitempty"`
	LabelSelector string       `json:"label_selector,omitempty"`
}

type resolvedEndpoint struct {
	Endpoint TraceEndpoint
	Pods     []spcgk8s.PodDetail
	IPNode   *ipEndpointNode
}

type ipEndpointNode struct {
	ID       string
	Label    string
	Kind     string
	Detail   string
	External bool
}

func normalizeDiscoverRequest(req *DiscoverRequest) error {
	if req.Source.Mode == "" && len(req.Selections) > 0 {
		req.Source = selectionToEndpoint(req.Selections[0])
	}
	if req.Source.Mode == "" {
		return fmt.Errorf("source endpoint is required")
	}
	if req.Destination.Mode == "" {
		req.Destination = TraceEndpoint{
			Mode:     EndpointIP,
			IP:       "external",
			External: true,
		}
	}
	return nil
}

func selectionToEndpoint(s spcgk8s.CaptureSelection) TraceEndpoint {
	ep := TraceEndpoint{Mode: EndpointNamespace, Namespace: s.Namespace}
	switch strings.ToLower(s.Type) {
	case "owner", "workload", "controller":
		ep.Type = "owner"
		ep.OwnerKind = s.OwnerKind
		ep.OwnerName = s.OwnerName
		ep.LabelSelector = s.LabelSelector
	default:
		ep.Type = "pod"
		ep.PodName = s.PodName
		ep.PodUID = s.PodUID
	}
	return ep
}

func endpointToSelections(ep TraceEndpoint) ([]spcgk8s.CaptureSelection, error) {
	if ep.Mode != EndpointNamespace {
		return nil, fmt.Errorf("namespace endpoint required")
	}
	if ep.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	switch strings.ToLower(ep.Type) {
	case "owner", "workload", "controller", "":
		if ep.OwnerKind == "" || ep.OwnerName == "" {
			return nil, fmt.Errorf("owner_kind and owner_name are required for workload selection")
		}
		return []spcgk8s.CaptureSelection{{
			Namespace: ep.Namespace, Type: "owner",
			OwnerKind: ep.OwnerKind, OwnerName: ep.OwnerName,
			LabelSelector: ep.LabelSelector,
		}}, nil
	case "pod":
		if ep.PodName == "" {
			return nil, fmt.Errorf("pod_name is required for pod selection")
		}
		return []spcgk8s.CaptureSelection{{
			Namespace: ep.Namespace, Type: "pod",
			PodName: ep.PodName, PodUID: ep.PodUID,
		}}, nil
	default:
		return nil, fmt.Errorf("unknown endpoint type %q", ep.Type)
	}
}

func resolveEndpoint(ctx context.Context, cs kubernetes.Interface, ep TraceEndpoint, searchNamespaces []string) (resolvedEndpoint, error) {
	out := resolvedEndpoint{Endpoint: ep}
	switch ep.Mode {
	case EndpointNamespace:
		pods, err := resolveNamespaceEndpoint(ctx, cs, ep)
		if err != nil {
			return out, err
		}
		out.Pods = pods
	case EndpointIP:
		ipNode, pods, err := resolveIPEndpoint(ctx, cs, ep, searchNamespaces)
		if err != nil {
			return out, err
		}
		out.IPNode = ipNode
		out.Pods = pods
	default:
		return out, fmt.Errorf("unknown endpoint mode %q", ep.Mode)
	}
	return out, nil
}

func resolveNamespaceEndpoint(ctx context.Context, cs kubernetes.Interface, ep TraceEndpoint) ([]spcgk8s.PodDetail, error) {
	sels, err := endpointToSelections(ep)
	if err != nil {
		return nil, err
	}
	resolved, err := spcgk8s.ResolveCaptureSelections(ctx, cs, sels)
	if err != nil {
		return nil, err
	}
	if len(resolved.Pods) == 0 {
		return nil, fmt.Errorf("no pods resolved for %s/%s", ep.Namespace, endpointLabel(ep))
	}
	return resolved.Pods, nil
}

func resolveIPEndpoint(ctx context.Context, cs kubernetes.Interface, ep TraceEndpoint, searchNamespaces []string) (*ipEndpointNode, []spcgk8s.PodDetail, error) {
	ip := strings.TrimSpace(ep.IP)
	if ip == "" || strings.EqualFold(ip, "external") {
		return &ipEndpointNode{
			ID:       nodeID("external", "", "destination"),
			Label:    "External",
			Kind:     "external",
			Detail:   "destination IP",
			External: true,
		}, nil, nil
	}
	if net.ParseIP(ip) == nil {
		return nil, nil, fmt.Errorf("invalid IP address %q", ip)
	}
	matched := findPodsByIP(ctx, cs, ip, searchNamespaces)
	external := ep.External
	kind := "external"
	label := ip
	detail := "external IP"
	if len(matched) > 0 {
		kind = "pod-ip"
		detail = matched[0].Namespace + "/" + matched[0].Name
		external = false
	} else if !external {
		external = !isPrivateIP(ip)
	}
	return &ipEndpointNode{
		ID:       nodeID("ip", "", ip),
		Label:    label,
		Kind:     kind,
		Detail:   detail,
		External: external,
	}, matched, nil
}

func findPodsByIP(ctx context.Context, cs kubernetes.Interface, ip string, namespaces []string) []spcgk8s.PodDetail {
	var out []spcgk8s.PodDetail
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		list, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for i := range list.Items {
			p := &list.Items[i]
			if podHasIP(p, ip) {
				out = append(out, spcgk8s.PodToDetail(ctx, cs, p))
			}
		}
	}
	return out
}

func podHasIP(p *corev1.Pod, ip string) bool {
	if strings.TrimSpace(p.Status.PodIP) == ip {
		return true
	}
	for _, pip := range p.Status.PodIPs {
		if strings.TrimSpace(pip.IP) == ip {
			return true
		}
	}
	return false
}

func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast()
}

func endpointLabel(ep TraceEndpoint) string {
	if ep.Mode == EndpointIP {
		return ep.IP
	}
	if ep.Type == "pod" && ep.PodName != "" {
		return ep.PodName
	}
	if ep.OwnerKind != "" {
		return ep.OwnerKind + "/" + ep.OwnerName
	}
	return ep.Namespace
}

func namespacesForScope(requested []string, pods ...[]spcgk8s.PodDetail) map[string]struct{} {
	out := map[string]struct{}{}
	for _, ns := range requested {
		if ns = strings.TrimSpace(ns); ns != "" {
			out[ns] = struct{}{}
		}
	}
	for _, group := range pods {
		for _, p := range group {
			if p.Namespace != "" {
				out[p.Namespace] = struct{}{}
			}
		}
	}
	return out
}
