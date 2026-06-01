package capture

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
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

	log.Printf("capture stream session=%s targets=%d port=%d", sessionID, len(targets), port)

	sess, err := e.SensorMgr.StartSession(ctx, sessionID, port, sensorTargets)
	if err != nil {
		log.Printf("capture session=%s sensor start failed: %v", sessionID, err)
		return status.Errorf(codes.Internal, "failed starting netobserv eBPF sensors: %v", err)
	}
	log.Printf("capture session=%s sensor ready ds=%s collector_port=%d", sessionID, sess.DaemonSet, port)
	e.sessions.Store(sessionID, sess)
	defer func() {
		_ = e.SensorMgr.StopSession(context.Background(), sess)
		e.sessions.Delete(sessionID)
	}()

	rawCh := make(chan sensor.PacketRecord, 4096)
	errCh := make(chan error, 1)
	go func() {
		errCh <- sess.Collector.StreamContext(ctx, rawCh)
	}()

	var seq, cum, windowBytes, pps uint64
	var received, forwarded, skippedEmpty uint64
	lastWindow := time.Now()
	lastStats := time.Now()

	sendPodRefresh := func(ev sensor.PodRefreshEvent) error {
		type podRow struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			UID       string `json:"uid"`
			OwnerKind string `json:"owner_kind,omitempty"`
			PodIP     string `json:"pod_ip,omitempty"`
		}
		rows := make([]podRow, 0, len(ev.Pods))
		for _, p := range ev.Pods {
			kind := ""
			if p.PrimaryOwner != nil {
				kind = p.PrimaryOwner.Kind
			}
			rows = append(rows, podRow{
				Namespace: p.Namespace, Name: p.Name, UID: p.UID,
				OwnerKind: kind, PodIP: p.PodIP,
			})
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"event": "pod_refresh",
			"pods":  rows,
		})
		seq++
		return stream.Send(&capturev1.CaptureChunk{
			SessionId:       sessionID,
			PodName:         "capture-targets",
			StitchedRestart: true,
			Sequence:        seq,
			FlowMetadata:    string(payload),
		})
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-sess.RefreshCh:
			if !ok {
				continue
			}
			if err := sendPodRefresh(ev); err != nil {
				return fmt.Errorf("failed sending pod refresh: %w", err)
			}
		case err := <-errCh:
			if err != nil && err != context.Canceled {
				return status.Errorf(codes.Internal, "netobserv collector stream ended: %v", err)
			}
			return nil
		case rec, ok := <-rawCh:
			if !ok {
				return nil
			}
			received++
			if len(rec.Data) == 0 {
				skippedEmpty++
				continue
			}
			// Pod scoping is enforced upstream in netobserv eBPF FLOW_FILTER_RULES.
			forwarded++
			data := rec.Data
			now := time.Now()
			if now.Sub(lastWindow) >= time.Second {
				pps = windowBytes
				windowBytes = 0
				lastWindow = now
			}
			windowBytes += uint64(len(data))
			cum += uint64(len(data))
			seq++

			var metaJSON string
			if len(rec.Meta) > 0 {
				if b, err := json.Marshal(rec.Meta); err == nil {
					metaJSON = string(b)
				}
			}

			podLabel := aggregatePodName(targets)
			if ns, name, ok := sensor.CapturePodFromMeta(rec.Meta, sess.TrackedPods); ok {
				podLabel = ns + "/" + name
			}
			chunk := &capturev1.CaptureChunk{
				SessionId:       sessionID,
				PodName:         podLabel,
				Data:            data,
				Sequence:        seq,
				PacketsPerSec:   pps,
				CumulativeBytes: cum,
				FlowMetadata:    metaJSON,
			}
			if err := stream.Send(chunk); err != nil {
				return fmt.Errorf("failed sending capture chunk: %w", err)
			}
			if now := time.Now(); now.Sub(lastStats) >= 10*time.Second {
				log.Printf("capture session=%s stats received=%d forwarded=%d skipped_empty=%d cum_bytes=%d",
					sessionID, received, forwarded, skippedEmpty, cum)
				lastStats = now
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
