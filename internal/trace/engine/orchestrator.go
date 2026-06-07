package engine

import (
	"context"
	"fmt"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"

	"k8s.io/client-go/rest"
)

// Engine orchestrates cross-layer trace discovery (logical → infrastructure → physical).
type Engine struct {
	Catalog *trace.Catalog
	Access  spcgk8s.NamespaceAccess

	OVN OVNClient
	OVS OVSFlowDumper

	// Simulated enables offline OVN/OVS synthesis when SBDB/SSH are unavailable.
	Simulated bool
}

// Config wires cluster clients into a trace Engine.
type Config struct {
	Cluster   *spcgk8s.Cluster
	Access    spcgk8s.NamespaceAccess
	OVN       OVNClient
	OVS       OVSFlowDumper
	Simulated bool
}

// NewEngine builds an Engine from cluster clients and optional plane backends.
func NewEngine(cfg Config) (*Engine, error) {
	if cfg.Cluster == nil || cfg.Cluster.Interface == nil {
		return nil, fmt.Errorf("kubernetes cluster client is required")
	}
	cat, err := trace.OpenCatalog(cfg.Cluster.Interface, cfg.Cluster.REST)
	if err != nil {
		return nil, err
	}
	ovn := cfg.OVN
	if ovn == nil {
		if cfg.Simulated {
			ovn = SimulatedOVNClient{}
		} else {
			ovn = StubOVNClient{}
		}
	}
	return &Engine{
		Catalog:   cat,
		Access:    cfg.Access,
		OVN:       ovn,
		OVS:       cfg.OVS,
		Simulated: cfg.Simulated,
	}, nil
}

// NewEngineFromREST is a convenience constructor for portal handlers.
func NewEngineFromREST(cs *spcgk8s.ClientsetWrap, cfg *rest.Config, access spcgk8s.NamespaceAccess) (*Engine, error) {
	cluster, err := spcgk8s.FromClientsetWrap(cs, cfg)
	if err != nil {
		return nil, err
	}
	return NewEngine(Config{Cluster: cluster, Access: access, Simulated: true})
}

// TracePath executes the four-layer discovery pipeline for src→dst endpoints.
func (e *Engine) TracePath(ctx context.Context, req trace.DiscoverRequest) (*TopologyResult, error) {
	if e == nil || e.Catalog == nil {
		return nil, fmt.Errorf("trace engine is not configured")
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	resp, err := e.Catalog.Resolve(ctx, req)
	if err != nil {
		return nil, err
	}

	logical := newLogicalDiscoverer(e.Catalog).discover(ctx, req, resp)
	nodeNames := collectNodeNames(resp)

	infraDisc := infrastructureDiscoverer{ovn: e.OVN, ovs: e.OVS}
	infra := infraDisc.discover(ctx, logical, nodeNames)

	physDisc := physicalDiscoverer{dc: dynamicAdapter{dc: e.Catalog.DC}}
	physical := physDisc.discover(ctx, resp, infra)

	result := mergeTopology(req, resp, logical, infra, physical)
	return result, nil
}

func validateRequest(req trace.DiscoverRequest) error {
	if len(req.Namespaces) == 0 {
		return fmt.Errorf("namespaces are required")
	}
	if req.Source.Mode == "" {
		return fmt.Errorf("source endpoint is required")
	}
	if req.Destination.Mode == "" {
		return fmt.Errorf("destination endpoint is required")
	}
	return nil
}

func mergeTopology(req trace.DiscoverRequest, resp *trace.DiscoverResponse, logical LogicalPlaneResult, infra InfrastructurePlaneResult, physical PhysicalPlaneResult) *TopologyResult {
	nodes := make([]TopologyNode, 0, len(logical.Nodes)+len(infra.Nodes)+len(physical.Nodes))
	edges := make([]TopologyEdge, 0, len(logical.Edges)+len(infra.Edges)+len(physical.Edges))
	edgeStates := map[string]EdgeVerificationState{}

	appendPlane := func(ns []TopologyNode, es []TopologyEdge) {
		nodes = append(nodes, ns...)
		for _, edge := range es {
			edges = append(edges, edge)
			if edge.State == "" {
				edge.State = EdgeTheoryOnly
			}
			edgeStates[edge.ID] = edge.State
		}
	}
	appendPlane(logical.Nodes, logical.Edges)
	appendPlane(infra.Nodes, infra.Edges)
	appendPlane(physical.Nodes, physical.Edges)

	layers := VisualizationLayers{
		Logical:        LayerRegion{Label: "Logical", Anchor: "top-left", X: 0, Y: 0, Width: resp.Graph.Width, Height: resp.Graph.Height / 3},
		Infrastructure: LayerRegion{Label: "Infrastructure", Anchor: "middle", X: 0, Y: resp.Graph.Height / 3, Width: resp.Graph.Width, Height: resp.Graph.Height / 3},
		Physical:       LayerRegion{Label: "Physical", Anchor: "bottom-right", X: 0, Y: 2 * resp.Graph.Height / 3, Width: resp.Graph.Width, Height: resp.Graph.Height / 3},
	}

	return &TopologyResult{
		TraceID:         req.TraceID,
		Source:          req.Source,
		Destination:     req.Destination,
		Graph:           resp.Graph,
		Logical:         logical,
		Infrastructure:  infra,
		Physical:        physical,
		Nodes:           nodes,
		Edges:           edges,
		EdgeStates:      edgeStates,
		Layers:          layers,
	}
}
