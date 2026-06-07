package k8s

import (
	"context"
	"fmt"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NamespaceAccess summarizes RBAC-visible namespace scope for a user token.
type NamespaceAccess struct {
	Allowed map[string]struct{}
	// ClusterInfraVisible is true when the caller can list nodes cluster-wide (infra detail allowed).
	ClusterInfraVisible bool
}

// ListAccessibleNamespaces returns namespaces the user can get/list pods in.
func ListAccessibleNamespaces(ctx context.Context, cs kubernetes.Interface) ([]string, error) {
	if cs == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		name := strings.TrimSpace(ns.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// VerifyNamespaces ensures every requested namespace is accessible to the user.
func VerifyNamespaces(ctx context.Context, cs kubernetes.Interface, requested []string) (NamespaceAccess, error) {
	access := NamespaceAccess{Allowed: map[string]struct{}{}}
	if cs == nil {
		return access, fmt.Errorf("kubernetes client is required")
	}
	allowed, err := ListAccessibleNamespaces(ctx, cs)
	if err != nil {
		return access, err
	}
	for _, ns := range allowed {
		access.Allowed[ns] = struct{}{}
	}
	for _, ns := range requested {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		if _, ok := access.Allowed[ns]; !ok {
			return access, fmt.Errorf("namespace %q is not accessible with current RBAC", ns)
		}
	}
	access.ClusterInfraVisible = canListNodes(ctx, cs)
	return access, nil
}

func canListNodes(ctx context.Context, cs kubernetes.Interface) bool {
	review, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "list",
				Group:    "",
				Version:  "v1",
				Resource: "nodes",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return false
	}
	return review.Status.Allowed
}

// VerifyEndpointNamespaces checks source/destination endpoint namespaces against access.
func VerifyEndpointNamespaces(ctx context.Context, cs kubernetes.Interface, namespaces []string, sourceNS, destNS string) error {
	access, err := VerifyNamespaces(ctx, cs, namespaces)
	if err != nil {
		return err
	}
	for _, ns := range []string{sourceNS, destNS} {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		if _, ok := access.Allowed[ns]; !ok {
			return fmt.Errorf("endpoint namespace %q is outside RBAC scope", ns)
		}
	}
	return nil
}

// PodNamespace returns the namespace for a pod reference (helper for handlers).
func PodNamespace(p *corev1.Pod) string {
	if p == nil {
		return ""
	}
	return p.Namespace
}
