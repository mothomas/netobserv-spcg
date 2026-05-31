package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type PodDetail struct {
	Namespace      string     `json:"namespace"`
	Name           string     `json:"name"`
	UID            string     `json:"uid"`
	Status         string     `json:"status"`
	PodIP          string     `json:"pod_ip"`
	NodeName       string     `json:"node_name"`
	LabelSelector  string     `json:"label_selector,omitempty"`
	Owners         []OwnerRef `json:"owners"`
	PrimaryOwner   *OwnerRef  `json:"primary_owner,omitempty"`
}

type ControllerSummary struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Status        string `json:"status"`
	Ready         string `json:"ready,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
}

type NamespaceWorkloads struct {
	Namespace    string              `json:"namespace"`
	Pods         []PodDetail         `json:"pods"`
	Deployments  []ControllerSummary `json:"deployments"`
	StatefulSets []ControllerSummary `json:"statefulsets"`
	DaemonSets   []ControllerSummary `json:"daemonsets"`
}

func ListWorkloadsAcrossNamespaces(ctx context.Context, cs kubernetes.Interface, namespaces []string) ([]NamespaceWorkloads, error) {
	out := make([]NamespaceWorkloads, 0, len(namespaces))
	for _, ns := range namespaces {
		w, err := ListNamespaceWorkloads(ctx, cs, ns)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, nil
}

func ListNamespaceWorkloads(ctx context.Context, cs kubernetes.Interface, ns string) (*NamespaceWorkloads, error) {
	result := &NamespaceWorkloads{Namespace: ns}

	pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed listing pods in namespace %s: %w", ns, err)
	}
	for i := range pods.Items {
		result.Pods = append(result.Pods, podToDetail(ctx, cs, &pods.Items[i]))
	}

	deps, err := cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed listing deployments in %s: %w", ns, err)
	}
	for _, d := range deps.Items {
		result.Deployments = append(result.Deployments, controllerSummary(ns, "Deployment", d.Name, d.Status.AvailableReplicas, d.Spec.Replicas, metav1.FormatLabelSelector(d.Spec.Selector)))
	}

	sts, err := cs.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed listing statefulsets in %s: %w", ns, err)
	}
	for _, st := range sts.Items {
		result.StatefulSets = append(result.StatefulSets, controllerSummary(ns, "StatefulSet", st.Name, st.Status.ReadyReplicas, st.Spec.Replicas, metav1.FormatLabelSelector(st.Spec.Selector)))
	}

	ds, err := cs.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed listing daemonsets in %s: %w", ns, err)
	}
	for _, d := range ds.Items {
		st := "Pending"
		if d.Status.NumberReady == d.Status.DesiredNumberScheduled && d.Status.DesiredNumberScheduled > 0 {
			st = "Running"
		}
		result.DaemonSets = append(result.DaemonSets, ControllerSummary{
			Kind: "DaemonSet", Name: d.Name, Namespace: ns, Status: st,
			Ready: fmt.Sprintf("%d/%d", d.Status.NumberReady, d.Status.DesiredNumberScheduled),
			LabelSelector: metav1.FormatLabelSelector(d.Spec.Selector),
		})
	}
	return result, nil
}

func podToDetail(ctx context.Context, cs kubernetes.Interface, p *corev1.Pod) PodDetail {
	owners := make([]OwnerRef, 0, len(p.OwnerReferences))
	for _, o := range p.OwnerReferences {
		owners = append(owners, OwnerRef{Kind: o.Kind, Name: o.Name, UID: string(o.UID)})
	}
	primary := enrichPrimaryOwner(ctx, cs, p)
	if primary == nil && len(owners) > 0 {
		primary = &owners[0]
	}
	return PodDetail{
		Namespace:     p.Namespace,
		Name:          p.Name,
		UID:           string(p.UID),
		Status:        string(p.Status.Phase),
		PodIP:         p.Status.PodIP,
		NodeName:      p.Spec.NodeName,
		LabelSelector: labelsFromPod(p),
		Owners:        owners,
		PrimaryOwner:  primary,
	}
}

func labelsFromPod(p *corev1.Pod) string {
	for _, o := range p.OwnerReferences {
		if o.Kind == "ReplicaSet" {
			// common pod template labels are on the pod itself
			if app, ok := p.Labels["app"]; ok {
				return "app=" + app
			}
			if app, ok := p.Labels["app.kubernetes.io/name"]; ok {
				return "app.kubernetes.io/name=" + app
			}
		}
	}
	if app, ok := p.Labels["app"]; ok {
		return "app=" + app
	}
	return ""
}

func controllerSummary(ns, kind, name string, ready int32, replicas *int32, selector string) ControllerSummary {
	want := int32(1)
	if replicas != nil {
		want = *replicas
	}
	st := "Pending"
	if ready >= want && want > 0 {
		st = "Running"
	} else if ready == 0 && want > 0 {
		st = "Failed"
	}
	return ControllerSummary{
		Kind: kind, Name: name, Namespace: ns, Status: st,
		Ready: fmt.Sprintf("%d/%d", ready, want), LabelSelector: selector,
	}
}

