package probe

import (
	"sync"
)

// captureLink routes capture session packets into an active probe session.
type captureLink struct {
	traceID string
	icmpID  uint16
}

var (
	captureMu    sync.Mutex
	captureLinks = map[string]*captureLink{} // capture_session_id -> link
)

// LinkCaptureSession binds a trace capture session to the active probe paint session.
func LinkCaptureSession(captureSessionID, traceID string, icmpID uint16) {
	if captureSessionID == "" || traceID == "" {
		return
	}
	captureMu.Lock()
	captureLinks[captureSessionID] = &captureLink{traceID: traceID, icmpID: icmpID}
	captureMu.Unlock()
}

// UnlinkCaptureSession removes a capture→probe binding.
func UnlinkCaptureSession(captureSessionID string) {
	if captureSessionID == "" {
		return
	}
	captureMu.Lock()
	delete(captureLinks, captureSessionID)
	captureMu.Unlock()
}

// ObserveCapturePacket ingests one capture frame for active probe correlation.
func ObserveCapturePacket(captureSessionID string, frame []byte, meta map[string]interface{}, seq uint64) {
	if captureSessionID == "" {
		return
	}
	captureMu.Lock()
	link := captureLinks[captureSessionID]
	captureMu.Unlock()
	if link == nil {
		return
	}
	s, ok := getSession(link.traceID)
	if !ok || s == nil {
		return
	}
	s.ingestCapture(frame, meta, seq, link.icmpID)
}
