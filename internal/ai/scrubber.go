package ai

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
)

var (
	emailRe   = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	bearerRe  = regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9\-._~+/]+=*`)
	uuidRe    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	accountRe = regexp.MustCompile(`(?i)(account[_-]?id|org[_-]?id)\s*[:=]\s*["']?([a-zA-Z0-9\-_]{6,})["']?`)
	macRe     = regexp.MustCompile(`(?i)\b([0-9a-f]{2}(:[0-9a-f]{2}){5})\b`)
	ipRe      = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// Scrubber applies deterministic tokenization with a reversible map for display.
type Scrubber struct {
	mu      sync.Mutex
	ipMap   map[string]string
	idMap   map[string]string
	macMap  map[string]string
	reverse map[string]string
}

func NewScrubber() *Scrubber {
	return &Scrubber{
		ipMap:   make(map[string]string),
		idMap:   make(map[string]string),
		macMap:  make(map[string]string),
		reverse: make(map[string]string),
	}
}

// Scrub masks sensitive values in free text.
func (s *Scrubber) Scrub(input string) string {
	out := input
	out = bearerRe.ReplaceAllString(out, "${1}<REDACTED_BEARER>")
	out = emailRe.ReplaceAllStringFunc(out, s.maskID)
	out = uuidRe.ReplaceAllStringFunc(out, s.maskID)
	out = accountRe.ReplaceAllString(out, "${1}<SANITIZED_ID>")
	out = macRe.ReplaceAllStringFunc(out, s.maskMAC)
	out = s.replaceIPs(out)
	return out
}

// Restore replaces scrub tokens with originals for operator-facing text.
func (s *Scrubber) Restore(input string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := input
	for token, orig := range s.reverse {
		out = strings.ReplaceAll(out, token, orig)
	}
	return out
}

// ScrubJSONLMap scrubs string values in a flat map (flow metadata).
func (s *Scrubber) ScrubJSONLMap(m map[string]interface{}) {
	for k, v := range m {
		switch t := v.(type) {
		case string:
			m[k] = s.Scrub(t)
		}
	}
}

// ScrubStringFields scrubs arbitrary string fields.
func (s *Scrubber) ScrubStringFields(vals ...*string) {
	for _, p := range vals {
		if p != nil && *p != "" {
			*p = s.Scrub(*p)
		}
	}
}

// SnapshotMap returns token→original for UI legend (no secrets in values shown to user — tokens only).
func (s *Scrubber) SnapshotMap() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.reverse))
	for k, v := range s.reverse {
		out[k] = v
	}
	return out
}

func (s *Scrubber) replaceIPs(input string) string {
	return ipRe.ReplaceAllStringFunc(input, func(ip string) string {
		if net.ParseIP(ip) == nil {
			return ip
		}
		return s.maskIP(ip)
	})
}

func (s *Scrubber) maskIP(ip string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.ipMap[ip]; ok {
		return v
	}
	n := len(s.ipMap) + 1
	tag := fmt.Sprintf("<INTERNAL_IP_%d>", n)
	s.ipMap[ip] = tag
	s.reverse[tag] = ip
	return tag
}

func (s *Scrubber) maskID(val string) string {
	return s.maskToken(val, "SANITIZED_ID")
}

func (s *Scrubber) maskMAC(val string) string {
	return s.maskToken(val, "MAC")
}

func (s *Scrubber) maskToken(val, kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.idMap
	if kind == "MAC" {
		m = s.macMap
	}
	if v, ok := m[val]; ok {
		return v
	}
	n := len(m) + 1
	tag := fmt.Sprintf("<%s_%d>", kind, n)
	m[val] = tag
	s.reverse[tag] = val
	return tag
}

// session scrubbers keyed by capture session id
var (
	scrubMu       sync.Mutex
	sessionScrub  = make(map[string]*Scrubber)
)

func ScrubberForSession(sessionID string) *Scrubber {
	scrubMu.Lock()
	defer scrubMu.Unlock()
	if s, ok := sessionScrub[sessionID]; ok {
		return s
	}
	s := NewScrubber()
	sessionScrub[sessionID] = s
	return s
}

func DropScrubber(sessionID string) {
	scrubMu.Lock()
	delete(sessionScrub, sessionID)
	scrubMu.Unlock()
}
