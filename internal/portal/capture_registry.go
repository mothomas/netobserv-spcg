package portal

import (
	"context"
	"sync"
	"time"

	"github.com/netobserv/spcg/internal/ai"
	"github.com/netobserv/spcg/internal/pcap"
)

var (
	captureOwnerMu       sync.Mutex
	captureOwners        = make(map[string]string) // capture session id -> auth session id
	activeCaptureStreams = make(map[string]struct{}) // sessions with an open SSE ingest stream
)

func registerCaptureSession(captureID, authSessionID string) {
	if captureID == "" || authSessionID == "" {
		return
	}
	captureOwnerMu.Lock()
	captureOwners[captureID] = authSessionID
	captureOwnerMu.Unlock()
}

func markCaptureStreamActive(captureID string) {
	if captureID == "" {
		return
	}
	captureOwnerMu.Lock()
	activeCaptureStreams[captureID] = struct{}{}
	captureOwnerMu.Unlock()
}

// releaseCaptureStream frees an admission slot when the SSE client disconnects.
// Capture data remains available until explicit teardown.
func releaseCaptureStream(captureID string) {
	if captureID == "" {
		return
	}
	captureOwnerMu.Lock()
	delete(activeCaptureStreams, captureID)
	captureOwnerMu.Unlock()
}

func activeCaptureSessionCount() int {
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	return len(activeCaptureStreams)
}

func storedCaptureSessionCount() int {
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	return len(captureOwners)
}

func releaseAllCaptureStreamsForAuth(authSessionID string) int {
	if authSessionID == "" {
		return 0
	}
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	n := 0
	for cid, owner := range captureOwners {
		if owner == authSessionID {
			if _, active := activeCaptureStreams[cid]; active {
				delete(activeCaptureStreams, cid)
				n++
			}
		}
	}
	return n
}

func assertCaptureOwner(captureID, authSessionID string) bool {
	if captureID == "" || authSessionID == "" {
		return false
	}
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	owner, ok := captureOwners[captureID]
	return ok && owner == authSessionID
}

// purgeCaptureSessions removes PCAP/AI state for all captures owned by an auth session.
func purgeCaptureSessions(authSessionID string) []string {
	if authSessionID == "" {
		return nil
	}
	var ids []string
	captureOwnerMu.Lock()
	for cid, owner := range captureOwners {
		if owner == authSessionID {
			ids = append(ids, cid)
		}
	}
	captureOwnerMu.Unlock()

	for _, cid := range ids {
		teardownCaptureSession(cid)
	}
	return ids
}

func teardownCaptureSession(captureID string) {
	if sess, ok := pcap.Get(captureID); ok && sess.S3Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, _ = sess.FinalizeS3(ctx)
		cancel()
	}
	if captureGraph != nil && captureGraph.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = captureGraph.DeleteCapture(ctx, captureID)
		cancel()
	}
	pcap.Delete(captureID)
	ai.DropScrubber(captureID)
	chatHistMu.Lock()
	delete(chatHist, captureID)
	chatHistMu.Unlock()
	aiSessionsMu.Lock()
	if c, ok := aiSessions[captureID]; ok {
		if len(c.bearer) > 0 {
			for i := range c.bearer {
				c.bearer[i] = 0
			}
		}
		delete(aiSessions, captureID)
	}
	aiSessionsMu.Unlock()
	captureOwnerMu.Lock()
	delete(captureOwners, captureID)
	delete(activeCaptureStreams, captureID)
	captureOwnerMu.Unlock()
}

var captureGraph *GraphStore

// SetCaptureGraph wires the Neo4j store for session teardown cleanup.
func SetCaptureGraph(g *GraphStore) {
	captureGraph = g
}
