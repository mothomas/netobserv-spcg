package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	capturev1 "github.com/netobserv/spcg/api/proto/capture/v1"
	"github.com/netobserv/spcg/internal/capture/sensor"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/pcap"
	"github.com/netobserv/spcg/internal/trace/probe"
	"google.golang.org/grpc"
)

type captureIngestResult struct {
	Session  *pcap.Session
	Resolved *spcgk8s.ResolvedCapture
}

func (s *Server) prepareCaptureIngest(
	ctx context.Context,
	authSID string,
	resolved *spcgk8s.ResolvedCapture,
	sessNS string,
	s3cfg pcap.S3CaptureConfig,
) (*captureIngestResult, error) {
	if resolved == nil || len(resolved.Pods) == 0 {
		return nil, fmt.Errorf("no pods resolved for capture")
	}
	limits := s.captureLimits()
	if err := limits.ValidateStart(len(resolved.Pods), activeCaptureSessionCount(), s3cfg.Enabled); err != nil {
		return nil, err
	}

	var sess *pcap.Session
	var err error
	if s3cfg.Enabled {
		if err := s3cfg.ValidForCapture(); err != nil {
			return nil, err
		}
		sess, err = pcap.NewSessionWithS3(ctx, sessNS, s3cfg)
	} else {
		sess = pcap.NewSession(sessNS)
	}
	if err != nil {
		return nil, err
	}

	tracked := make([]pcap.TrackedPod, 0, len(resolved.Pods))
	for _, p := range resolved.Pods {
		kind := ""
		if p.PrimaryOwner != nil {
			kind = p.PrimaryOwner.Kind
		}
		tracked = append(tracked, pcap.TrackedPod{
			Namespace: p.Namespace, Name: p.Name, UID: p.UID, OwnerKind: kind,
			PodIP: p.PodIP, PodIPs: append([]string(nil), p.PodIPs...),
		})
	}
	sess.SetTrackedPods(tracked)
	pcap.Store(sess)
	registerCaptureSession(sess.ID, authSID)
	markCaptureStreamActive(sess.ID)

	return &captureIngestResult{Session: sess, Resolved: resolved}, nil
}

func (s *Server) runCaptureIngest(ctx context.Context, prep *captureIngestResult) {
	if prep == nil || prep.Session == nil {
		return
	}
	sess := prep.Session
	resolved := prep.Resolved
	limits := s.captureLimits()

	defer func() {
		releaseCaptureStream(sess.ID)
		if sess.S3Enabled() {
			finCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			_, _ = sess.FinalizeS3(finCtx)
			cancel()
		}
	}()

	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(s.EngineTLS)}
	conn, err := grpc.NewClient(s.EngineAddr, dialOpts...)
	if err != nil {
		return
	}
	defer conn.Close()

	client := capturev1.NewCaptureServiceClient(conn)
	stream, err := client.StreamPackets(ctx)
	if err != nil {
		return
	}

	for _, t := range resolved.SensorTargets {
		tr := &capturev1.TargetPodRequest{
			SessionId: sess.ID, Namespace: t.Namespace,
			PodName: t.PodName, PodUid: t.PodUID,
			WorkloadKind: t.WorkloadKind, WorkloadName: t.WorkloadName,
			LabelSelector: t.LabelSelector, Port: t.Port,
		}
		if err := stream.Send(tr); err != nil {
			return
		}
	}
	if err := stream.CloseSend(); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		chunk, err := stream.Recv()
		if err != nil {
			return
		}
		meta := chunk.GetFlowMetadata()
		if chunk.GetStitchedRestart() && meta != "" {
			var refresh struct {
				Event string                   `json:"event"`
				Pods  []map[string]interface{} `json:"pods"`
			}
			if json.Unmarshal([]byte(meta), &refresh) == nil && refresh.Event == "pod_refresh" {
				nt := make([]pcap.TrackedPod, 0, len(refresh.Pods))
				for _, p := range refresh.Pods {
					ns, _ := p["namespace"].(string)
					name, _ := p["name"].(string)
					uid, _ := p["uid"].(string)
					kind, _ := p["owner_kind"].(string)
					if ns != "" && name != "" {
						podIP, _ := p["pod_ip"].(string)
						nt = append(nt, pcap.TrackedPod{
							Namespace: ns, Name: name, UID: uid, OwnerKind: kind, PodIP: podIP,
						})
					}
				}
				if len(nt) > 0 {
					sess.SetTrackedPods(nt)
				}
			}
		}
		podName, podUID := chunk.GetPodName(), chunk.GetPodUid()
		if meta != "" {
			var fm map[string]interface{}
			if json.Unmarshal([]byte(meta), &fm) == nil {
				if ns, name, ok := sensor.CapturePodFromMeta(sensor.FlowMetadata(fm), trackedPodsFromSession(sess)); ok {
					podName = ns + "/" + name
				}
			}
		}
		sess.AppendFlow(podName, podUID, chunk.GetData(), meta, chunk.GetSequence())
		probe.ObserveCapturePacket(sess.ID, chunk.GetData(), parseCaptureMeta(meta), chunk.GetSequence())
		if err := sess.LastS3Error(); err != nil {
			return
		}
		if reason, stop := limits.ShouldStopCapture(sess); stop {
			_ = reason
			return
		}
	}
}

func parseCaptureMeta(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return nil
	}
	return m
}
