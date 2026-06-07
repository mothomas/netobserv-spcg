package probe

import (
	"context"
	"sync"
)

type subscriber struct {
	ch chan ProbeEvent
}

type session struct {
	traceID    string
	probeID    string
	corr       *GraphCorrelator
	cancel     context.CancelFunc
	subscribers map[*subscriber]struct{}
	subMu      sync.Mutex
}

var (
	sessMu sync.Mutex
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
	if s.cancel != nil {
		s.cancel()
	}
	s.broadcast(ProbeEvent{Type: "probe_finished", TraceID: traceID, ProbeID: s.probeID})
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
