package probe

import (
	"fmt"
	"hash/fnv"
)

// PaintToken derives a stable ICMP identifier and human token from a trace id.
func PaintToken(traceID string) (token string, icmpID uint16) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(traceID))
	icmpID = uint16(h.Sum32() & 0xffff)
	if icmpID == 0 {
		icmpID = 1
	}
	token = fmt.Sprintf("spcg:%04x", icmpID)
	return token, icmpID
}
