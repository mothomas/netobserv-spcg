package sensor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
)

// ExtractPacketBytes decodes netobserv packet capture payloads (base64 Data field).
func ExtractPacketBytes(flow *genericmap.Flow) ([]byte, error) {
	if flow == nil || flow.GenericMap == nil {
		return nil, fmt.Errorf("empty flow from netobserv collector")
	}
	var gm config.GenericMap
	if err := json.Unmarshal(flow.GenericMap.Value, &gm); err != nil {
		return nil, fmt.Errorf("failed parsing netobserv generic map: %w", err)
	}
	raw, ok := gm["Data"]
	if !ok {
		return nil, nil
	}
	s, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("netobserv Data field is not a string")
	}
	return base64.StdEncoding.DecodeString(s)
}
