package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CaptureSelection is a user-selected capture subject (pod and/or workload owner).
type CaptureSelection struct {
	Namespace     string `json:"namespace"`
	Type          string `json:"type"` // "pod" or "owner"
	PodName       string `json:"pod_name,omitempty"`
	PodUID        string `json:"pod_uid,omitempty"`
	OwnerKind     string `json:"owner_kind,omitempty"`
	OwnerName     string `json:"owner_name,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
	Port          int32  `json:"port,omitempty"`
}

// ResolvedCapture is the expanded target set sent to the capture engine.
type ResolvedCapture struct {
	SensorTargets []SensorTarget `json:"sensor_targets"`
	Pods          []PodDetail    `json:"pods"`
}

// SensorTarget is the engine/netobserv filter unit.
type SensorTarget struct {
	Namespace     string
	PodName       string
	PodUID        string
	WorkloadKind  string
	WorkloadName  string
	LabelSelector string
	Port          int32
}

func ResolveCaptureSelections(ctx context.Context, cs kubernetes.Interface, selections []CaptureSelection) (*ResolvedCapture, error) {
	if len(selections) == 0 {
		return nil, fmt.Errorf("no capture selections provided")
	}

	ownerKeys := map[string]struct{}{}
	var owners []CaptureSelection
	var pods []CaptureSelection

	for _, s := range selections {
		switch strings.ToLower(s.Type) {
		case "owner", "workload", "controller":
			if s.Namespace == "" || s.OwnerKind == "" || s.OwnerName == "" {
				return nil, fmt.Errorf("owner selection requires namespace, owner_kind, and owner_name")
			}
			key := ownerKey(s.Namespace, s.OwnerKind, s.OwnerName)
			if _, ok := ownerKeys[key]; ok {
				continue
			}
			ownerKeys[key] = struct{}{}
			owners = append(owners, s)
		case "pod", "":
			if s.Namespace == "" || s.PodName == "" {
				return nil, fmt.Errorf("pod selection requires namespace and pod_name")
			}
			pods = append(pods, s)
		default:
			return nil, fmt.Errorf("unknown selection type %q", s.Type)
		}
	}

	out := &ResolvedCapture{}
	seenPods := map[string]struct{}{}

	for _, o := range owners {
		selector, err := o.labelSelectorOrLookup(ctx, cs)
		if err != nil {
			return nil, err
		}
		out.SensorTargets = append(out.SensorTargets, SensorTarget{
			Namespace: o.Namespace, WorkloadKind: o.OwnerKind, WorkloadName: o.OwnerName,
			LabelSelector: selector, Port: o.Port,
		})

		matched, err := listPodsByOwner(ctx, cs, o.Namespace, o.OwnerKind, o.OwnerName, selector)
		if err != nil {
			return nil, err
		}
		for _, p := range matched {
			if _, ok := seenPods[p.UID]; ok {
				continue
			}
			seenPods[p.UID] = struct{}{}
			out.Pods = append(out.Pods, p)
		}
	}

	for _, p := range pods {
		if coveredByOwnerSelection(ctx, cs, p, owners) {
			continue
		}
		detail, err := getPodDetail(ctx, cs, p.Namespace, p.PodName)
		if err != nil {
			return nil, err
		}
		if _, ok := seenPods[detail.UID]; ok {
			continue
		}
		seenPods[detail.UID] = struct{}{}
		out.Pods = append(out.Pods, detail)
		out.SensorTargets = append(out.SensorTargets, SensorTarget{
			Namespace: p.Namespace, PodName: detail.Name, PodUID: detail.UID,
			LabelSelector: detail.LabelSelector, Port: p.Port,
		})
	}

	if len(out.SensorTargets) == 0 {
		return nil, fmt.Errorf("no capture targets resolved from selections")
	}
	return out, nil
}

func (s CaptureSelection) labelSelectorOrLookup(ctx context.Context, cs kubernetes.Interface) (string, error) {
	if s.LabelSelector != "" {
		return s.LabelSelector, nil
	}
	return controllerLabelSelector(ctx, cs, s.Namespace, s.OwnerKind, s.OwnerName)
}

func ownerKey(ns, kind, name string) string {
	return ns + "/" + kind + "/" + name
}

func coveredByOwnerSelection(ctx context.Context, cs kubernetes.Interface, p CaptureSelection, owners []CaptureSelection) bool {
	if len(owners) == 0 {
		return false
	}
	detail, err := getPodDetail(ctx, cs, p.Namespace, p.PodName)
	if err != nil {
		return false
	}
	for _, o := range owners {
		if o.Namespace != detail.Namespace {
			continue
		}
		if podMatchesOwner(&detail, o.OwnerKind, o.OwnerName) {
			return true
		}
	}
	return false
}

func listPodsByOwner(ctx context.Context, cs kubernetes.Interface, ns, kind, name, selector string) ([]PodDetail, error) {
	if selector == "" {
		var err error
		selector, err = controllerLabelSelector(ctx, cs, ns, kind, name)
		if err != nil {
			return nil, err
		}
	}
	opts := metav1.ListOptions{LabelSelector: selector}
	list, err := cs.CoreV1().Pods(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed listing pods for owner %s/%s in %s: %w", kind, name, ns, err)
	}
	out := make([]PodDetail, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		detail := podToDetail(ctx, cs, p)
		if podMatchesOwner(&detail, kind, name) {
			out = append(out, detail)
		}
	}
	return out, nil
}

func podMatchesOwner(p *PodDetail, kind, name string) bool {
	if p.PrimaryOwner != nil && p.PrimaryOwner.Kind == kind && p.PrimaryOwner.Name == name {
		return true
	}
	for _, o := range p.Owners {
		if o.Kind == kind && o.Name == name {
			return true
		}
	}
	return false
}

func getPodDetail(ctx context.Context, cs kubernetes.Interface, ns, name string) (PodDetail, error) {
	p, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return PodDetail{}, fmt.Errorf("failed getting pod %s/%s: %w", ns, name, err)
	}
	return podToDetail(ctx, cs, p), nil
}
