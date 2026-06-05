package portal

import (
	"sync"
	"time"

	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	"github.com/netobserv/spcg/internal/trace"
)

type traceSession struct {
	AuthSessionID string
	Response      trace.DiscoverResponse
	SigmaGraph    *graphdb.SigmaGraph
	CreatedAt     time.Time
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
	delete(traceOwners, traceID)
	traceOwnerMu.Unlock()
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
