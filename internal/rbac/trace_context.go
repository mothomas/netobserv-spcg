package rbac

import (
	"context"
	"fmt"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"
	"github.com/netobserv/spcg/internal/trace/engine"

	"k8s.io/client-go/kubernetes"
)

// TraceContext carries RBAC scope for trace discovery and response sanitization.
type TraceContext struct {
	AuthSessionID       string
	Access              spcgk8s.NamespaceAccess
	SanitizeInfra       bool
}

// NewTraceContext builds trace RBAC context from namespace verification.
func NewTraceContext(ctx context.Context, cs kubernetes.Interface, authSessionID string, namespaces []string) (TraceContext, error) {
	access, err := spcgk8s.VerifyNamespaces(ctx, cs, namespaces)
	if err != nil {
		return TraceContext{}, err
	}
	return TraceContext{
		AuthSessionID: authSessionID,
		Access:        access,
		SanitizeInfra: !access.ClusterInfraVisible,
	}, nil
}

// ValidateEndpoints ensures workload endpoints stay inside allowed namespaces.
func (tc TraceContext) ValidateEndpoints(src, dst engine.Endpoint) error {
	check := func(label string, ep engine.Endpoint) error {
		if ep.Mode != trace.EndpointNamespace {
			return nil
		}
		ns := ep.Namespace
		if ns == "" {
			return fmt.Errorf("%s namespace is required", label)
		}
		if _, ok := tc.Access.Allowed[ns]; !ok {
			return fmt.Errorf("%s namespace %q is outside RBAC scope", label, ns)
		}
		return nil
	}
	if err := check("source", src); err != nil {
		return err
	}
	return check("destination", dst)
}
