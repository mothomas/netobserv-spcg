package ebpf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// CiliumHookAttacher is a stub attacher that records hook targets without loading BPF objects.
// Replace Attach* bodies with cilium/ebpf program load + link.Attach when BPF objects ship.
type CiliumHookAttacher struct {
	attached []string
}

func NewCiliumHookAttacher() *CiliumHookAttacher {
	return &CiliumHookAttacher{}
}

func (h *CiliumHookAttacher) AttachVethEgress(_ context.Context, netnsInum uint32, ifIndex int32) error {
	h.attached = append(h.attached, fmt.Sprintf("tc:veth:ns=%d:if=%d", netnsInum, ifIndex))
	return nil
}

func (h *CiliumHookAttacher) AttachSecondaryInterface(_ context.Context, ifIndex int32, hook HookPoint) error {
	h.attached = append(h.attached, fmt.Sprintf("tc:secondary:%s:if=%d", hook, ifIndex))
	return nil
}

func (h *CiliumHookAttacher) AttachOVSDatapath(_ context.Context) error {
	// Stubs for kprobes: ovs_vport_receive, do_execute_actions
	h.attached = append(h.attached, "kprobe:ovs_vport_receive", "kprobe:do_execute_actions")
	return nil
}

func (h *CiliumHookAttacher) AttachPhysicalEgress(_ context.Context, ifIndex int32) error {
	h.attached = append(h.attached, fmt.Sprintf("tc:egress:phys:if=%d", ifIndex))
	return nil
}

func (h *CiliumHookAttacher) DetachAll(context.Context) error {
	h.attached = nil
	return nil
}

func (h *CiliumHookAttacher) Attached() []string {
	out := make([]string, len(h.attached))
	copy(out, h.attached)
	return out
}

// DiscoverHostIfIndex resolves a host interface name to ifindex (stub uses netlink in production).
func DiscoverHostIfIndex(iface string) (int32, error) {
	if iface == "" {
		return 0, fmt.Errorf("interface name is required")
	}
	// Placeholder: production uses netlink.LinkByName.
	return int32(len(iface)), nil
}

// DiscoverPodVethIfIndex locates the host-side veth ifindex for a pod netns (stub).
func DiscoverPodVethIfIndex(netnsPath string) (int32, uint32, error) {
	if netnsPath == "" {
		return 0, 0, fmt.Errorf("netns path is required")
	}
	inum := uint32(len(filepath.Clean(netnsPath)))
	if _, err := os.Stat(netnsPath); err != nil {
		return 0, 0, err
	}
	return int32(inum % 1000), inum, nil
}

// RingBufferLoop drains a RingBufferReader and sends decoded events to out.
func RingBufferLoop(ctx context.Context, reader RingBufferReader, out chan<- FlowEvent) error {
	defer reader.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		raw, err := reader.Read()
		if err != nil {
			return err
		}
		ev, err := DecodeRawFlowRecord(raw)
		if err != nil {
			continue
		}
		select {
		case out <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
