package sensor

import (
	"context"
	"fmt"
	"sync"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
)

// PacketCollector receives packet flows from netobserv eBPF agents via flowlogs-pipeline gRPC.
type PacketCollector struct {
	port   int
	flows  chan *genericmap.Flow
	server *grpc.CollectorServer
	wg     sync.WaitGroup
}

func StartPacketCollector(port int) (*PacketCollector, error) {
	flows := make(chan *genericmap.Flow, 256)
	srv, err := grpc.StartCollector(port, flows)
	if err != nil {
		return nil, fmt.Errorf("failed starting flowlogs-pipeline grpc collector on port %d: %w", port, err)
	}
	return &PacketCollector{port: port, flows: flows, server: srv}, nil
}

func (c *PacketCollector) Port() int { return c.port }

func (c *PacketCollector) Flows() <-chan *genericmap.Flow { return c.flows }

func (c *PacketCollector) Close() {
	if c.server != nil {
		_ = c.server.Close()
	}
}

// StreamContext pumps decoded PCAP chunks to the output channel until ctx is done.
func (c *PacketCollector) StreamContext(ctx context.Context, out chan<- []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case flow, ok := <-c.flows:
			if !ok {
				return nil
			}
			data, err := ExtractPacketBytes(flow)
			if err != nil || len(data) == 0 {
				continue
			}
			select {
			case out <- data:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
