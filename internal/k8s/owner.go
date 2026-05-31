package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func controllerLabelSelector(ctx context.Context, cs kubernetes.Interface, ns, kind, name string) (string, error) {
	switch kind {
	case "Deployment":
		d, err := cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed getting deployment %s/%s: %w", ns, name, err)
		}
		return metav1.FormatLabelSelector(d.Spec.Selector), nil
	case "StatefulSet":
		s, err := cs.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed getting statefulset %s/%s: %w", ns, name, err)
		}
		return metav1.FormatLabelSelector(s.Spec.Selector), nil
	case "DaemonSet":
		d, err := cs.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed getting daemonset %s/%s: %w", ns, name, err)
		}
		return metav1.FormatLabelSelector(d.Spec.Selector), nil
	case "ReplicaSet":
		r, err := cs.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed getting replicaset %s/%s: %w", ns, name, err)
		}
		return metav1.FormatLabelSelector(r.Spec.Selector), nil
	default:
		return "", fmt.Errorf("unsupported owner kind for capture: %s", kind)
	}
}

func enrichPrimaryOwner(ctx context.Context, cs kubernetes.Interface, p *corev1.Pod) *OwnerRef {
	for _, o := range p.OwnerReferences {
		switch o.Kind {
		case "StatefulSet", "DaemonSet":
			return &OwnerRef{Kind: o.Kind, Name: o.Name, UID: string(o.UID)}
		case "ReplicaSet":
			rs, err := cs.AppsV1().ReplicaSets(p.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			for _, ro := range rs.OwnerReferences {
				if ro.Kind == "Deployment" {
					return &OwnerRef{Kind: "Deployment", Name: ro.Name, UID: string(ro.UID)}
				}
			}
			return &OwnerRef{Kind: "ReplicaSet", Name: o.Name, UID: string(o.UID)}
		}
	}
	return nil
}
