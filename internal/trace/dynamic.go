package trace

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// DynamicClient lists OpenShift/OVN/MetalLB CRDs with the user REST config.
type DynamicClient struct {
	client dynamic.Interface
}

func NewDynamicClient(cfg *rest.Config) (*DynamicClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rest config is required for CRD discovery")
	}
	dc, err := dynamic.NewForConfig(rest.CopyConfig(cfg))
	if err != nil {
		return nil, err
	}
	return &DynamicClient{client: dc}, nil
}

func (d *DynamicClient) list(ctx context.Context, gvr schema.GroupVersionResource, ns string) ([]unstructured.Unstructured, error) {
	if d == nil || d.client == nil {
		return nil, nil
	}
	var (
		list *unstructured.UnstructuredList
		err  error
	)
	if ns == "" {
		list, err = d.client.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		list, err = d.client.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
