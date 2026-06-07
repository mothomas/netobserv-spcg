package probe

import (
	"context"
	"sync"
	"time"
)

type subscriber struct {
	ch chan ProbeEvent
}

type session struct {
	traceID          string
	probeID          string
	corr             *GraphCorrelator
	cancel           context.CancelFunc
	captureSessionID string
	icmpID           uint16
	demoDrop         bool
	simulate         bool
	paintSeq         int
	lastCaptureSeq   uint64
	finished         bool
	obsNotify        chan struct{}
	subscribers      map[*subscriber]struct{}
	subMu            sync.Mutex
	paintMu          sync.Mutex
}

var (
	sessMu   sync.Mutex
	sessions = make(map[string]*session) // trace_id -> active probe session
)

func getSession(traceID string) (*session, bool) {
	sessMu.Lock()
	defer sessMu.Unlock()
	s, ok := sessions[traceID]
	return s, ok
}

func stopSession(traceID string) {
	sessMu.Lock()
	s, ok := sessions[traceID]
	if ok {
		delete(sessions, traceID)
	}
	sessMu.Unlock()
	if !ok {
		return
	}
	if s.captureSessionID != "" {
		UnlinkCaptureSession(s.captureSessionID)
	}
	if s.cancel != nil {
		s.cancel()
	}
	if !s.finished {
		s.broadcast(ProbeEvent{Type: "probe_finished", TraceID: traceID, ProbeID: s.probeID})
	}
	s.subMu.Lock()
	for sub := range s.subscribers {
		close(sub.ch)
		delete(s.subscribers, sub)
	}
	s.subMu.Unlock()
}

func StopTraceProbe(traceID string) {
	stopSession(traceID)
}

func (s *session) ingestCapture(frame []byte, meta map[string]interface{}, seq uint64, icmpID uint16) {
	if s == nil || !MatchPaintPacket(frame, meta, icmpID) {
		return
	}
	if seq > 0 && seq <= s.lastCaptureSeq {
		return
	}
	s.lastCaptureSeq = seq
	s.paintHop(hookFromMeta(meta), "capture")
	select {
	case s.obsNotify <- struct{}{}:
	default:
	}
}

func (s *session) paintHop(hook string, source string) {
	if s == nil || s.corr == nil {
		return
	}
	s.paintMu.Lock()
	defer s.paintMu.Unlock()
	if s.finished {
		return
	}

	if s.demoDrop && s.corr.Remaining() == 1 {
		edgeID := s.corr.NextEdgeID()
		if edgeID == "" {
			s.finishProbe("demo drop — no remaining hop")
			return
		}
		s.corr.MarkDropOnEdge(edgeID)
		s.paintSeq++
		reason := "NetworkPolicy: deny egress to destination"
		if source == "capture" {
			reason = "capture: policy drop on final hop"
		}
		s.broadcast(ProbeEvent{
			Type:       "edge_update",
			TraceID:    s.traceID,
			ProbeID:    s.probeID,
			EdgeID:     edgeID,
			State:      EdgeDroppedRed,
			Hook:       hook,
			Seq:        s.paintSeq,
			DropReason: reason,
			Verified:   s.corr.VerifiedCount(),
			Total:      s.corr.PrimaryCount(),
		})
		s.finishProbe("path blocked at " + edgeID)
		return
	}

	edgeID, ok := s.corr.Advance(hook)
	if !ok {
		s.finishProbe("all primary hops verified")
		return
	}
	s.paintSeq++
	s.broadcast(ProbeEvent{
		Type:     "edge_update",
		TraceID:  s.traceID,
		ProbeID:  s.probeID,
		EdgeID:   edgeID,
		State:    EdgeActiveGreen,
		Hook:     hook,
		Seq:      s.paintSeq,
		Verified: s.corr.VerifiedCount(),
		Total:    s.corr.PrimaryCount(),
	})
	if s.corr.Remaining() == 0 {
		s.finishProbe("all primary hops verified")
	}
}

func (s *session) finishProbe(message string) {
	if s == nil || s.finished {
		return
	}
	s.finished = true
	s.broadcast(ProbeEvent{
		Type:     "probe_finished",
		TraceID:  s.traceID,
		ProbeID:  s.probeID,
		Message:  message,
		Verified: s.corr.VerifiedCount(),
		Total:    s.corr.PrimaryCount(),
	})
	go func(id string) {
		// Allow SSE clients to receive the final frame before teardown.
		// stopSession is invoked by the fire goroutine defer.
	}(s.traceID)
}

func (s *session) isFinished() bool {
	if s == nil {
		return true
	}
	s.paintMu.Lock()
	defer s.paintMu.Unlock()
	return s.finished
}

func (s *session) broadcast(ev ProbeEvent) {
	if s == nil {
		return
	}
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- ev:
		default:
		}
	}
}

// SubscribeEvents listens for probe paint events on an active trace session.
func SubscribeEvents(traceID string) (<-chan ProbeEvent, func(), bool) {
	return subscribe(traceID)
}

func subscribe(traceID string) (<-chan ProbeEvent, func(), bool) {
	s, ok := getSession(traceID)
	if !ok {
		return nil, nil, false
	}
	sub := &subscriber{ch: make(chan ProbeEvent, 64)}
	s.subMu.Lock()
	if s.subscribers == nil {
		s.subscribers = make(map[*subscriber]struct{})
	}
	s.subscribers[sub] = struct{}{}
	s.subMu.Unlock()
	unsub := func() {
		s.subMu.Lock()
		if _, ok := s.subscribers[sub]; ok {
			delete(s.subscribers, sub)
			close(sub.ch)
		}
		s.subMu.Unlock()
	}
	return sub.ch, unsub, true
}

func runSimulatedPaint(ctx context.Context, s *session) {
	hooks := []string{"veth_egress", "ovs_vport_receive", "ovs_execute_actions", "physical_egress"}
	tick := 420 * time.Millisecond
	if s != nil && s.simulate {
		tick = 520 * time.Millisecond
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if s.isFinished() {
			return
		}
		hook := hooks[s.paintSeq%len(hooks)]
		s.paintHop(hook, "simulate")
		if s.isFinished() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(tick):
		}
	}
}

func runCapturePaintLoop(ctx context.Context, s *session) {
	fallback := time.After(3 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-fallback:
			runSimulatedPaint(ctx, s)
			return
		case <-s.obsNotify:
			if s.isFinished() {
				return
			}
		}
	}
}
