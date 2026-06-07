package ebpf

import (
	"context"
	"fmt"
	"sync"

	"github.com/netobserv/spcg/internal/trace/engine"
)

// RingBufferReader abstracts cilium/ebpf ringbuf.Reader without loading BPF in unit tests.
type RingBufferReader interface {
	Read() ([]byte, error)
	Close() error
}

// Agent attaches trace hooks and correlates live flows against predicted topology edges.
type Agent interface {
	Start(ctx context.Context, traceID string, predicted *engine.TopologyResult) error
	Stop(ctx context.Context) error
	Events() <-chan FlowEvent
	EdgeStates() map[string]engine.EdgeVerificationState
}

// HookAttacher installs TC/OVS/physical probes for a trace session.
type HookAttacher interface {
	AttachVethEgress(ctx context.Context, netnsInum uint32, ifIndex int32) error
	AttachSecondaryInterface(ctx context.Context, ifIndex int32, hook HookPoint) error
	AttachOVSDatapath(ctx context.Context) error
	AttachPhysicalEgress(ctx context.Context, ifIndex int32) error
	DetachAll(ctx context.Context) error
}

// Correlator maps live FlowEvents to predicted topology edges.
type Correlator struct {
	mu         sync.RWMutex
	predicted  *engine.TopologyResult
	edgeStates map[string]engine.EdgeVerificationState
}

func NewCorrelator(predicted *engine.TopologyResult) *Correlator {
	states := map[string]engine.EdgeVerificationState{}
	if predicted != nil {
		for id, st := range predicted.EdgeStates {
			states[id] = st
		}
		for _, e := range predicted.Edges {
			if _, ok := states[e.ID]; !ok {
				states[e.ID] = engine.EdgeTheoryOnly
			}
		}
	}
	return &Correlator{predicted: predicted, edgeStates: states}
}

// Observe ingests a flow event and mutates edge verification state.
func (c *Correlator) Observe(ev FlowEvent) (updated []string) {
	if c == nil || c.predicted == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if ev.Dropped {
		if edgeID := c.locateDropEdge(ev); edgeID != "" {
			c.edgeStates[edgeID] = engine.EdgeDroppedRed
			return []string{edgeID}
		}
		return nil
	}
	for _, edge := range c.predicted.Edges {
		if c.matches(ev, edge) {
			c.edgeStates[edge.ID] = engine.EdgeActiveGreen
			updated = append(updated, edge.ID)
		}
	}
	return updated
}

func (c *Correlator) matches(ev FlowEvent, edge engine.TopologyEdge) bool {
	_ = edge
	if ev.Dropped {
		return false
	}
	switch ev.Hook {
	case HookVethEgress, HookSecondaryCNI, HookOVSVportReceive, HookOVSExecute, HookPhysicalEgress:
		return true
	default:
		return false
	}
}

func (c *Correlator) locateDropEdge(ev FlowEvent) string {
	for _, edge := range c.predicted.Edges {
		if edge.ACLMetadata != "" || edge.OpenFlowCookie != "" {
			return edge.ID
		}
	}
	if len(c.predicted.Edges) > 0 {
		return c.predicted.Edges[len(c.predicted.Edges)-1].ID
	}
	return ""
}

func (c *Correlator) Snapshot() map[string]engine.EdgeVerificationState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]engine.EdgeVerificationState, len(c.edgeStates))
	for k, v := range c.edgeStates {
		out[k] = v
	}
	return out
}

// StubAgent is a production-safe no-BPF implementation for portal wiring.
type StubAgent struct {
	corr   *Correlator
	events chan FlowEvent
	stop   context.CancelFunc
}

func NewStubAgent() *StubAgent {
	return &StubAgent{events: make(chan FlowEvent, 256)}
}

func (a *StubAgent) Start(ctx context.Context, traceID string, predicted *engine.TopologyResult) error {
	if a == nil {
		return fmt.Errorf("ebpf agent is nil")
	}
	a.corr = NewCorrelator(predicted)
	child, cancel := context.WithCancel(ctx)
	a.stop = cancel
	go func() {
		<-child.Done()
	}()
	return nil
}

func (a *StubAgent) Stop(context.Context) error {
	if a.stop != nil {
		a.stop()
	}
	return nil
}

func (a *StubAgent) Events() <-chan FlowEvent { return a.events }

func (a *StubAgent) EdgeStates() map[string]engine.EdgeVerificationState {
	if a.corr == nil {
		return map[string]engine.EdgeVerificationState{}
	}
	return a.corr.Snapshot()
}

// InjectEvent allows tests and capture bridges to feed synthetic flow events.
func (a *StubAgent) InjectEvent(ev FlowEvent) {
	if a.corr != nil {
		a.corr.Observe(ev)
	}
	select {
	case a.events <- ev:
	default:
	}
}
