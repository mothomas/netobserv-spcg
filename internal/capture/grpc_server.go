package capture

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"sync"
	"time"

	capturev1 "github.com/netobserv/spcg/api/proto/capture/v1"
	"github.com/netobserv/spcg/internal/capture/sensor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
)

type EngineServer struct {
	capturev1.UnimplementedCaptureServiceServer
	Client    kubernetes.Interface
	SensorMgr *sensor.Manager
	sessions  sync.Map
}

func NewEngineServer(client kubernetes.Interface) *EngineServer {
	return &EngineServer{
		Client:    client,
		SensorMgr: sensor.NewManager(client),
	}
}

func (e *EngineServer) StreamPackets(stream capturev1.CaptureService_StreamPacketsServer) error {
	ctx := stream.Context()

	targets, sessionID, err := e.collectTargets(stream)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed reading capture targets: %v", err)
	}
	if sessionID == "" {
		return status.Errorf(codes.InvalidArgument, "session_id is required")
	}

	port := sessionPort(sessionID)
	sensorTargets := make([]sensor.Target, 0, len(targets))
	for _, t := range targets {
		sensorTargets = append(sensorTargets, sensor.Target{
			SessionID: sessionID, Namespace: t.GetNamespace(),
			PodName: t.GetPodName(), PodUID: t.GetPodUid(),
			WorkloadKind: t.GetWorkloadKind(), WorkloadName: t.GetWorkloadName(),
			LabelSelector: t.GetLabelSelector(), Port: t.GetPort(),
		})
	}

	sess, err := e.SensorMgr.StartSession(ctx, sessionID, port, sensorTargets)
	if err != nil {
		return status.Errorf(codes.Internal, "failed starting netobserv eBPF sensors: %v", err)
	}
	e.sessions.Store(sessionID, sess)
	defer func() {
		_ = e.SensorMgr.StopSession(context.Background(), sess)
		e.sessions.Delete(sessionID)
	}()

	rawCh := make(chan []byte, 128)
	errCh := make(chan error, 1)
	go func() {
		errCh <- sess.Collector.StreamContext(ctx, rawCh)
	}()

	var seq, cum, windowBytes, pps uint64
	lastWindow := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && err != context.Canceled {
				return status.Errorf(codes.Internal, "netobserv collector stream ended: %v", err)
			}
			return nil
		case data, ok := <-rawCh:
			if !ok {
				return nil
			}
			now := time.Now()
			if now.Sub(lastWindow) >= time.Second {
				pps = windowBytes
				windowBytes = 0
				lastWindow = now
			}
			windowBytes += uint64(len(data))
			cum += uint64(len(data))
			seq++

			chunk := &capturev1.CaptureChunk{
				SessionId:       sessionID,
				PodName:         aggregatePodName(targets),
				Data:            data,
				Sequence:        seq,
				PacketsPerSec:   pps,
				CumulativeBytes: cum,
			}
			if err := stream.Send(chunk); err != nil {
				return fmt.Errorf("failed sending capture chunk: %w", err)
			}
		}
	}
}

func (e *EngineServer) collectTargets(stream capturev1.CaptureService_StreamPacketsServer) ([]*capturev1.TargetPodRequest, string, error) {
	var targets []*capturev1.TargetPodRequest
	var sessionID string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if sessionID == "" {
			sessionID = req.GetSessionId()
		}
		targets = append(targets, req)
	}

	if len(targets) == 0 {
		return nil, "", fmt.Errorf("no targets received")
	}
	return targets, sessionID, nil
}

func sessionPort(sessionID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sessionID))
	return 19000 + int(h.Sum32()%800)
}

func aggregatePodName(targets []*capturev1.TargetPodRequest) string {
	if len(targets) == 1 {
		if targets[0].GetWorkloadName() != "" {
			return targets[0].GetWorkloadKind() + "/" + targets[0].GetWorkloadName()
		}
		return targets[0].GetPodName()
	}
	return "multi"
}
