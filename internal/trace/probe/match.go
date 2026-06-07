package probe

import (
	"strconv"
	"strings"

	"github.com/netobserv/spcg/internal/pcap"
)

// MatchPaintPacket reports whether a capture observation carries the probe paint marker.
func MatchPaintPacket(frame []byte, meta map[string]interface{}, wantID uint16) bool {
	if wantID == 0 {
		return false
	}
	if id, ok := icmpIDFromMeta(meta); ok && id == wantID {
		return true
	}
	if id, ok := pcap.ICMPIdentifier(frame); ok && id == wantID {
		return true
	}
	return false
}

func icmpIDFromMeta(meta map[string]interface{}) (uint16, bool) {
	if len(meta) == 0 {
		return 0, false
	}
	for _, key := range []string{"IcmpId", "ICMPId", "icmp_id", "IcmpID"} {
		if v, ok := meta[key]; ok {
			if id, ok := parseUint16(v); ok {
				return id, true
			}
		}
	}
	proto := strings.ToUpper(strings.TrimSpace(metaString(meta, "Proto")))
	if proto != "ICMP" && proto != "ICMPV6" {
		return 0, false
	}
	if id, ok := parseUint16(meta["SrcPort"]); ok {
		return id, true
	}
	if id, ok := parseUint16(meta["DstPort"]); ok {
		return id, true
	}
	return 0, false
}

func metaString(meta map[string]interface{}, key string) string {
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func parseUint16(v interface{}) (uint16, bool) {
	switch t := v.(type) {
	case float64:
		if t < 0 || t > 65535 {
			return 0, false
		}
		return uint16(t), true
	case int:
		if t < 0 || t > 65535 {
			return 0, false
		}
		return uint16(t), true
	case int64:
		if t < 0 || t > 65535 {
			return 0, false
		}
		return uint16(t), true
	case uint64:
		if t > 65535 {
			return 0, false
		}
		return uint16(t), true
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(t), 0, 16)
		if err != nil || n > 65535 {
			return 0, false
		}
		return uint16(n), true
	default:
		return 0, false
	}
}

func hookFromMeta(meta map[string]interface{}) string {
	for _, key := range []string{"Hook", "hook", "CaptureHook"} {
		if s := metaString(meta, key); s != "" {
			return s
		}
	}
	if pcap.DropCause(meta) != "" {
		return "policy_drop"
	}
	return "capture_observation"
}
