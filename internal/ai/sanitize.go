package ai

import (
	"net"
	"regexp"
	"sync"
)

var (
	emailRe    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	bearerRe   = regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9\-._~+/]+=*`)
	uuidRe     = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	accountRe  = regexp.MustCompile(`(?i)(account[_-]?id|org[_-]?id)\s*[:=]\s*["']?([a-zA-Z0-9\-_]{6,})["']?`)
	ipMapMu    sync.Mutex
	ipMap      = make(map[string]string)
	idMapMu    sync.Mutex
	idMap      = make(map[string]string)
)

// Sanitize applies Presidio-style deterministic masks to trace text.
func Sanitize(input string) string {
	out := input
	out = bearerRe.ReplaceAllString(out, "${1}<REDACTED_BEARER_AUTH>")
	out = emailRe.ReplaceAllStringFunc(out, maskID)
	out = uuidRe.ReplaceAllStringFunc(out, maskID)
	out = accountRe.ReplaceAllString(out, "${1}<SANITIZED_ID_1>")

	out = replaceIPs(out)
	return out
}

func replaceIPs(input string) string {
	return regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`).ReplaceAllStringFunc(input, func(ip string) string {
		if net.ParseIP(ip) == nil {
			return ip
		}
		return maskIP(ip)
	})
}

func maskIP(ip string) string {
	ipMapMu.Lock()
	defer ipMapMu.Unlock()
	if v, ok := ipMap[ip]; ok {
		return v
	}
	n := len(ipMap) + 1
	tag := "<INTERNAL_IP_" + itoa(n) + ">"
	ipMap[ip] = tag
	return tag
}

func maskID(s string) string {
	idMapMu.Lock()
	defer idMapMu.Unlock()
	if v, ok := idMap[s]; ok {
		return v
	}
	n := len(idMap) + 1
	tag := "<SANITIZED_ID_" + itoa(n) + ">"
	idMap[s] = tag
	return tag
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ResetMaps clears anonymization maps (call after session ends).
func ResetMaps() {
	ipMapMu.Lock()
	ipMap = make(map[string]string)
	ipMapMu.Unlock()
	idMapMu.Lock()
	idMap = make(map[string]string)
	idMapMu.Unlock()
}
