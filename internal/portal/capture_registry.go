package portal

import (
	"sync"

	"github.com/netobserv/spcg/internal/ai"
	"github.com/netobserv/spcg/internal/pcap"
)

var (
	captureOwnerMu sync.Mutex
	captureOwners  = make(map[string]string) // capture session id -> auth session id
)

func registerCaptureSession(captureID, authSessionID string) {
	if captureID == "" || authSessionID == "" {
		return
	}
	captureOwnerMu.Lock()
	captureOwners[captureID] = authSessionID
	captureOwnerMu.Unlock()
}

func captureAuthOwner(captureID string) string {
	captureOwnerMu.Lock()
	defer captureOwnerMu.Unlock()
	return captureOwners[captureID]
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
	captureOwnerMu.Unlock()
}
