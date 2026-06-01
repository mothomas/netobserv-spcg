package sensor

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	pktgrpc "github.com/netobserv/netobserv-ebpf-agent/pkg/grpc/packet"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbpacket"
)

// PacketCollector receives PCA packets from netobserv eBPF agents (EXPORT=grpc packet API).
type PacketCollector struct {
	port      int
	packets   chan *pbpacket.Packet
	server    *pktgrpc.CollectorServer
	pktCount  atomic.Uint64
}

func StartPacketCollector(port int) (*PacketCollector, error) {
	packets := make(chan *pbpacket.Packet, 256)
	srv, err := pktgrpc.StartCollector(port, packets)
	if err != nil {
		return nil, fmt.Errorf("failed starting netobserv packet grpc collector on port %d: %w", port, err)
	}
	log.Printf("spcg-collector: netobserv packet grpc listening on :%d", port)
	return &PacketCollector{port: port, packets: packets, server: srv}, nil
}

func (c *PacketCollector) Port() int { return c.port }

func (c *PacketCollector) Close() {
	if c.server != nil {
		_ = c.server.Close()
	}
}

// StreamContext pumps PCA packets until ctx is done.
func (c *PacketCollector) StreamContext(ctx context.Context, out chan<- PacketRecord) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pkt, ok := <-c.packets:
			if !ok {
				return nil
			}
			n := c.pktCount.Add(1)
			if n == 1 || n%500 == 0 {
				log.Printf("spcg-collector: port=%d packets=%d", c.port, n)
			}
			data, err := ExtractPacketBytesFromPB(pkt)
			if err != nil || len(data) == 0 {
				continue
			}
			meta := FlowMetadataFromFrame(data)
			select {
			case out <- PacketRecord{Data: data, Meta: meta}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
