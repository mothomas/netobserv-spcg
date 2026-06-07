package portal

import (
	"context"
	"sync"
	"time"

	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	"github.com/netobserv/spcg/internal/trace"
)

type traceSession struct {
	AuthSessionID    string
	Response         trace.DiscoverResponse
	SigmaGraph       *graphdb.SigmaGraph
	CaptureSessionID string
	captureCancel    context.CancelFunc
	CreatedAt        time.Time
}

var (
	traceOwnerMu sync.Mutex
	traceOwners  = make(map[string]*traceSession) // trace_id -> session
)

func registerTraceSession(traceID, authSessionID string, resp trace.DiscoverResponse, sigma *graphdb.SigmaGraph) {
	if traceID == "" || authSessionID == "" {
		return
	}
	traceOwnerMu.Lock()
	traceOwners[traceID] = &traceSession{
		AuthSessionID: authSessionID,
		Response:      resp,
		SigmaGraph:    sigma,
		CreatedAt:     time.Now(),
	}
	traceOwnerMu.Unlock()
}

func getTraceSession(traceID string) (*traceSession, bool) {
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	s, ok := traceOwners[traceID]
	return s, ok
}

func assertTraceOwner(traceID, authSessionID string) bool {
	if traceID == "" || authSessionID == "" {
		return false
	}
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	s, ok := traceOwners[traceID]
	return ok && s.AuthSessionID == authSessionID
}

func deleteTraceSession(traceID string) {
	traceOwnerMu.Lock()
	if s, ok := traceOwners[traceID]; ok {
		if s.captureCancel != nil {
			s.captureCancel()
		}
	}
	delete(traceOwners, traceID)
	traceOwnerMu.Unlock()
}

func linkTraceCapture(traceID, captureID string, cancel context.CancelFunc) {
	if traceID == "" || captureID == "" {
		return
	}
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	s, ok := traceOwners[traceID]
	if !ok {
		if cancel != nil {
			cancel()
		}
		return
	}
	if s.captureCancel != nil {
		s.captureCancel()
	}
	s.CaptureSessionID = captureID
	s.captureCancel = cancel
}

func traceCaptureSessionID(traceID string) string {
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	s, ok := traceOwners[traceID]
	if !ok {
		return ""
	}
	return s.CaptureSessionID
}

func stopTraceCapture(traceID string) string {
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	s, ok := traceOwners[traceID]
	if !ok {
		return ""
	}
	captureID := s.CaptureSessionID
	if s.captureCancel != nil {
		s.captureCancel()
		s.captureCancel = nil
	}
	s.CaptureSessionID = ""
	return captureID
}

func purgeTraceSessions(authSessionID string) {
	if authSessionID == "" {
		return
	}
	traceOwnerMu.Lock()
	defer traceOwnerMu.Unlock()
	for id, s := range traceOwners {
		if s.AuthSessionID == authSessionID {
			delete(traceOwners, id)
		}
	}
}
