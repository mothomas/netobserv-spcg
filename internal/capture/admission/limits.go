package admission

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/spcg/internal/pcap"
)

// Limits are tier admission caps loaded from environment (ConfigMap in cluster).
type Limits struct {
	MaxConcurrentSessions int
	MaxPodsPerSession     int
	MaxCaptureDuration    time.Duration
	MaxCaptureBytes       uint64
	S3OffloadRequired     bool
}

// LoadFromEnv reads capture admission policy. Defaults match the Small tier in docs/architecture-tiers.md.
func LoadFromEnv() Limits {
	return Limits{
		MaxConcurrentSessions: envInt("MAX_CONCURRENT_SESSIONS", 2),
		MaxPodsPerSession:     envInt("MAX_PODS_PER_SESSION", 10),
		MaxCaptureDuration:    envDuration("MAX_CAPTURE_DURATION", 15*time.Minute),
		MaxCaptureBytes:       envBytes("MAX_CAPTURE_BYTES", 100*1024*1024),
		S3OffloadRequired:     envBool("S3_OFFLOAD_ENABLED", false),
	}
}

// Public returns a JSON-safe view for the UI.
func (l Limits) Public() map[string]interface{} {
	return map[string]interface{}{
		"max_concurrent_sessions": l.MaxConcurrentSessions,
		"max_pods_per_session":    l.MaxPodsPerSession,
		"max_capture_duration":    l.MaxCaptureDuration.String(),
		"max_capture_bytes":       l.MaxCaptureBytes,
		"s3_offload_required":     l.S3OffloadRequired,
	}
}

// ValidateStart checks whether a new capture may begin under current tier policy.
func (l Limits) ValidateStart(resolvedPodCount, activeSessions int, s3Enabled bool) error {
	if l.MaxConcurrentSessions > 0 && activeSessions >= l.MaxConcurrentSessions {
		return fmt.Errorf("concurrent capture limit reached (%d); end an active session or scale tier", l.MaxConcurrentSessions)
	}
	if l.MaxPodsPerSession > 0 && resolvedPodCount > l.MaxPodsPerSession {
		return fmt.Errorf("capture selects %d pods; tier limit is %d per session", resolvedPodCount, l.MaxPodsPerSession)
	}
	if l.S3OffloadRequired && !s3Enabled {
		return fmt.Errorf("this deployment tier requires S3 streaming capture (enable S3 and test connection before start)")
	}
	return nil
}

// ShouldStopCapture enforces duration and RAM PCAP byte caps during an active session.
func (l Limits) ShouldStopCapture(sess *pcap.Session) (reason string, stop bool) {
	if sess == nil {
		return "", false
	}
	if l.MaxCaptureDuration > 0 && time.Since(sess.Created) > l.MaxCaptureDuration {
		return fmt.Sprintf("capture duration exceeded tier limit (%s)", l.MaxCaptureDuration), true
	}
	if !sess.S3Enabled() && l.MaxCaptureBytes > 0 && sess.TotalBytes() > l.MaxCaptureBytes {
		return fmt.Sprintf("capture size exceeded tier RAM limit (%s); use S3 streaming for larger captures", formatBytes(l.MaxCaptureBytes)), true
	}
	return "", false
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envBytes(key string, def uint64) uint64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if n, err := strconv.ParseUint(v, 10, 64); err == nil {
		return n
	}
	upper := strings.ToUpper(v)
	mult := uint64(1)
	switch {
	case strings.HasSuffix(upper, "KIB"):
		mult = 1024
		upper = strings.TrimSuffix(upper, "KIB")
	case strings.HasSuffix(upper, "MIB"):
		mult = 1024 * 1024
		upper = strings.TrimSuffix(upper, "MIB")
	case strings.HasSuffix(upper, "GIB"):
		mult = 1024 * 1024 * 1024
		upper = strings.TrimSuffix(upper, "GIB")
	case strings.HasSuffix(upper, "KB"):
		mult = 1000
		upper = strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "MB"):
		mult = 1000 * 1000
		upper = strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "GB"):
		mult = 1000 * 1000 * 1000
		upper = strings.TrimSuffix(upper, "GB")
	case strings.HasSuffix(upper, "KI"):
		mult = 1024
		upper = strings.TrimSuffix(upper, "KI")
	case strings.HasSuffix(upper, "MI"):
		mult = 1024 * 1024
		upper = strings.TrimSuffix(upper, "MI")
	case strings.HasSuffix(upper, "GI"):
		mult = 1024 * 1024 * 1024
		upper = strings.TrimSuffix(upper, "GI")
	}
	n, err := strconv.ParseUint(strings.TrimSpace(upper), 10, 64)
	if err != nil {
		return def
	}
	return n * mult
}

func formatBytes(n uint64) string {
	if n < 1024*1024 {
		return fmt.Sprintf("%d KiB", n/1024)
	}
	return fmt.Sprintf("%d MiB", n/(1024*1024))
}
