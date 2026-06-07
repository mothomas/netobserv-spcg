package trace

import (
	"context"
	"testing"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNormalizeDiscoverRequestLegacy(t *testing.T) {
	req := DiscoverRequest{
		Namespaces: []string{"demo"},
		Selections: []spcgk8s.CaptureSelection{{
			Namespace: "demo", Type: "pod", PodName: "curl-1",
		}},
	}
	if err := normalizeDiscoverRequest(&req); err != nil {
		t.Fatal(err)
	}
	if req.Source.Mode != EndpointNamespace || req.Source.PodName != "curl-1" {
		t.Fatalf("source: %+v", req.Source)
	}
	if req.Destination.Mode != EndpointIP {
		t.Fatalf("dest default: %+v", req.Destination)
	}
}

func TestResolveExternalDestIP(t *testing.T) {
	cs := fake.NewSimpleClientset()
	node, _, err := resolveIPEndpoint(context.Background(), cs, TraceEndpoint{Mode: EndpointIP, IP: "external", External: true}, []string{"demo"})
	if err != nil {
		t.Fatal(err)
	}
	if node == nil || !node.External {
		t.Fatalf("node: %+v", node)
	}
}

func TestResolveInvalidIP(t *testing.T) {
	cs := fake.NewSimpleClientset()
	_, _, err := resolveIPEndpoint(context.Background(), cs, TraceEndpoint{Mode: EndpointIP, IP: "not-an-ip"}, nil)
	if err == nil {
		t.Fatal("expected invalid IP error")
	}
}
