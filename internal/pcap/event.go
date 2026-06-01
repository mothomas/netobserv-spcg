package pcap

import (
	"encoding/json"
	"time"
)

// FlowEvent is one captured packet with netobserv enrichment and capture target context.
type FlowEvent struct {
	At           time.Time
	CapturePod   string
	CapturePodUID string
	Frame        []byte
	FlowMeta     map[string]interface{}
	Sequence     uint64
}

func parseFlowMeta(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return nil
	}
	return m
}
