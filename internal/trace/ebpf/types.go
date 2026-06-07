package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"
)

// FlowEvent is a normalized packet observation from TC/OVS/egress hooks.
type FlowEvent struct {
	SrcIP      net.IP `json:"src_ip"`
	DstIP      net.IP `json:"dst_ip"`
	SrcPort    uint16 `json:"src_port"`
	DstPort    uint16 `json:"dst_port"`
	Protocol   uint8  `json:"protocol"`
	NetNSInum  uint32 `json:"netns_inum"`
	IfIndex    int32  `json:"ifindex"`
	GeneveVNI  uint32 `json:"geneve_vni"`
	Dropped    bool   `json:"dropped"`
	DropReason string `json:"drop_reason,omitempty"`
	Hook       HookPoint `json:"hook"`
	TimestampNS int64 `json:"timestamp_ns"`
}

// HookPoint identifies where the probe fired in the datapath.
type HookPoint string

const (
	HookVethEgress      HookPoint = "veth_egress"
	HookSecondaryCNI    HookPoint = "secondary_cni"
	HookOVSVportReceive HookPoint = "ovs_vport_receive"
	HookOVSExecute      HookPoint = "ovs_execute_actions"
	HookPhysicalEgress  HookPoint = "physical_egress"
)

// RawFlowRecord is the BPF ring buffer wire layout (little-endian).
type RawFlowRecord struct {
	SrcIPBE    [16]byte
	DstIPBE    [16]byte
	SrcPort    uint16
	DstPort    uint16
	Protocol   uint8
	_          [3]byte
	NetNSInum  uint32
	IfIndex    int32
	GeneveVNI  uint32
	Flags      uint32 // bit0 dropped
	HookID     uint32
	TimestampNS int64
}

const rawFlowDroppedFlag = 1 << 0

// DecodeRawFlowRecord parses a BPF ring buffer sample into FlowEvent.
func DecodeRawFlowRecord(raw []byte) (FlowEvent, error) {
	if len(raw) < 64 {
		return FlowEvent{}, errShortRecord
	}
	rec := RawFlowRecord{
		SrcPort:     binary.LittleEndian.Uint16(raw[32:34]),
		DstPort:     binary.LittleEndian.Uint16(raw[34:36]),
		Protocol:    raw[36],
		NetNSInum:   binary.LittleEndian.Uint32(raw[40:44]),
		IfIndex:     int32(binary.LittleEndian.Uint32(raw[44:48])),
		GeneveVNI:   binary.LittleEndian.Uint32(raw[48:52]),
		TimestampNS: int64(binary.LittleEndian.Uint64(raw[56:64])),
	}
	copy(rec.SrcIPBE[:], raw[0:16])
	copy(rec.DstIPBE[:], raw[16:32])
	flags := binary.LittleEndian.Uint32(raw[52:56])
	hookID := uint32(0)
	if len(raw) >= 68 {
		hookID = binary.LittleEndian.Uint32(raw[64:68])
	}
	return FlowEvent{
		SrcIP:       parseIP(rec.SrcIPBE[:]),
		DstIP:       parseIP(rec.DstIPBE[:]),
		SrcPort:     rec.SrcPort,
		DstPort:     rec.DstPort,
		Protocol:    rec.Protocol,
		NetNSInum:   rec.NetNSInum,
		IfIndex:     rec.IfIndex,
		GeneveVNI:   rec.GeneveVNI,
		Dropped:     flags&rawFlowDroppedFlag != 0,
		Hook:        hookFromID(hookID),
		TimestampNS: rec.TimestampNS,
	}, nil
}

func parseIP(b []byte) net.IP {
	if isZero(b[4:]) {
		return net.IP(b[:4])
	}
	return net.IP(b[:])
}

func isZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func hookFromID(id uint32) HookPoint {
	switch id {
	case 1:
		return HookVethEgress
	case 2:
		return HookSecondaryCNI
	case 3:
		return HookOVSVportReceive
	case 4:
		return HookOVSExecute
	case 5:
		return HookPhysicalEgress
	default:
		return HookVethEgress
	}
}

var errShortRecord = fmt.Errorf("flow record too short")
