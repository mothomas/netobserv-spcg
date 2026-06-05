package trace

import (
	"context"
	"testing"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCatalogResolveServiceAndPod(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "curl-1",
				Namespace: "demo",
				UID:       "uid-1",
			},
			Status: corev1.PodStatus{
				PodIP:    "10.0.0.5",
				Phase:    corev1.PodRunning,
				HostIP:   "192.168.1.10",
				PodIPs:   []corev1.PodIP{{IP: "10.0.0.5"}},
			},
			Spec: corev1.PodSpec{NodeName: "worker-1"},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "demo"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.0.1",
				Selector:  map[string]string{"app": "curl"},
				Ports:     []corev1.ServicePort{{Port: 8080}},
			},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-abc",
				Namespace: "demo",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "api"},
			},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.0.0.5"}},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(8080)}},
		},
	)

	cat := &Catalog{CS: cs}
	out, err := cat.Resolve(context.Background(), DiscoverRequest{
		Namespaces: []string{"demo"},
		Selections: []spcgk8s.CaptureSelection{{
			Namespace: "demo",
			Type:      "pod",
			PodName:   "curl-1",
		}},
		TraceID: "trace-test-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TargetPod.Name != "curl-1" {
		t.Fatalf("target pod: %+v", out.TargetPod)
	}
	if len(out.Graph.Nodes) < 3 {
		t.Fatalf("expected pod+service+node nodes, got %d", len(out.Graph.Nodes))
	}
	foundSvc := false
	for _, n := range out.Graph.Nodes {
		if n.Kind == "service-clusterip" && n.Label == "api" {
			foundSvc = true
		}
	}
	if !foundSvc {
		t.Fatalf("service node missing: %+v", out.Graph.Nodes)
	}
}

func ptrInt32(v int32) *int32 { return &v }
