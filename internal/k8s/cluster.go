package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Cluster bundles typed and dynamic Kubernetes clients used by trace discovery.
type Cluster struct {
	Interface kubernetes.Interface
	Dynamic   dynamic.Interface
	REST      *rest.Config
}

// NewCluster builds a Cluster from REST config (user-scoped or privileged).
func NewCluster(cfg *rest.Config) (*Cluster, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rest config is required")
	}
	cs, err := kubernetes.NewForConfig(rest.CopyConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	dc, err := dynamic.NewForConfig(rest.CopyConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return &Cluster{
		Interface: cs,
		Dynamic:   dc,
		REST:      rest.CopyConfig(cfg),
	}, nil
}

// FromClientsetWrap adapts an existing portal clientset + optional REST config.
func FromClientsetWrap(w *ClientsetWrap, cfg *rest.Config) (*Cluster, error) {
	if w == nil || w.Interface == nil {
		return nil, fmt.Errorf("kubernetes clientset is required")
	}
	out := &Cluster{Interface: w.Interface, REST: cfg}
	if cfg != nil {
		dc, err := dynamic.NewForConfig(rest.CopyConfig(cfg))
		if err != nil {
			return nil, err
		}
		out.Dynamic = dc
	}
	return out, nil
}
