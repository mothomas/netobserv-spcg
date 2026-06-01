package pcap

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/netobserv/spcg/internal/ai"
)

// JSONLRecord is one scrubbed packet line for upstream LLM analysis.
type JSONLRecord struct {
	Sequence   uint64                 `json:"sequence"`
	Timestamp  string                 `json:"timestamp"`
	CapturePod string                 `json:"capture_pod,omitempty"`
	CaptureUID string                 `json:"capture_pod_uid,omitempty"`
	Frame      FrameSummary           `json:"frame"`
	Flow       map[string]interface{} `json:"flow,omitempty"`
	K8s        map[string]string      `json:"k8s,omitempty"`
}

// ExportJSONL builds newline-delimited JSON with deterministic scrubbing via scrubber.
func (s *Session) ExportJSONL(scrub *ai.Scrubber, maxLines int) ([]byte, error) {
	events := s.Events()
	if len(events) == 0 {
		return nil, nil
	}
	if maxLines <= 0 {
		maxLines = 500
	}
	if len(events) > maxLines {
		events = events[len(events)-maxLines:]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ev := range events {
		rec := eventToRecord(ev)
		scrubRecord(scrub, &rec)
		if err := enc.Encode(rec); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func eventToRecord(ev FlowEvent) JSONLRecord {
	rec := JSONLRecord{
		Sequence:   ev.Sequence,
		Timestamp:  ev.At.UTC().Format(time.RFC3339Nano),
		CapturePod: ev.CapturePod,
		CaptureUID: ev.CapturePodUID,
		Frame:      summarizeFrame(ev.Frame),
	}
	if len(ev.FlowMeta) > 0 {
		rec.Flow = ev.FlowMeta
		rec.K8s = k8sView(ev.FlowMeta)
	}
	return rec
}

func scrubRecord(s *ai.Scrubber, rec *JSONLRecord) {
	if rec.Flow != nil {
		s.ScrubJSONLMap(rec.Flow)
	}
	if rec.K8s != nil {
		for k, v := range rec.K8s {
			rec.K8s[k] = s.Scrub(v)
		}
	}
	rec.CapturePod = s.Scrub(rec.CapturePod)
	rec.CaptureUID = s.Scrub(rec.CaptureUID)
	rec.Frame.DNSQuery = s.Scrub(rec.Frame.DNSQuery)
}

func k8sView(m map[string]interface{}) map[string]string {
	out := map[string]string{}
	set := func(k, key string) {
		if v, ok := m[k].(string); ok && v != "" {
			out[key] = v
		}
	}
	set("SrcK8S_Namespace", "src_namespace")
	set("SrcK8S_Name", "src_pod")
	set("SrcK8S_OwnerType", "src_owner_kind")
	set("SrcK8S_OwnerName", "src_owner_name")
	set("DstK8S_Namespace", "dst_namespace")
	set("DstK8S_Name", "dst_pod")
	set("DstK8S_OwnerType", "dst_owner_kind")
	set("DstK8S_OwnerName", "dst_owner_name")
	if len(out) == 0 {
		return nil
	}
	return out
}
