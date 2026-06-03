package admission

import (
	"testing"
	"time"

	"github.com/netobserv/spcg/internal/pcap"
)

func TestValidateStart(t *testing.T) {
	l := Limits{MaxConcurrentSessions: 2, MaxPodsPerSession: 10, S3OffloadRequired: true}
	if err := l.ValidateStart(5, 1, false); err == nil {
		t.Fatal("expected s3 required error")
	}
	if err := l.ValidateStart(5, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := l.ValidateStart(11, 0, true); err == nil {
		t.Fatal("expected pod limit error")
	}
}

func TestShouldStopCapture(t *testing.T) {
	l := Limits{MaxCaptureDuration: time.Minute, MaxCaptureBytes: 100}
	s := pcap.NewSession("ns")
	s.Created = time.Now().UTC().Add(-2 * time.Minute)
	if _, stop := l.ShouldStopCapture(s); !stop {
		t.Fatal("expected duration stop")
	}
	s2 := pcap.NewSession("ns")
	s2.Append("pod", "uid", make([]byte, 200))
	if _, stop := l.ShouldStopCapture(s2); !stop {
		t.Fatal("expected bytes stop")
	}
}

func TestEnvBytes(t *testing.T) {
	t.Setenv("MAX_CAPTURE_BYTES", "100Mi")
	if got := envBytes("MAX_CAPTURE_BYTES", 0); got != 100*1024*1024 {
		t.Fatalf("got %d", got)
	}
}
