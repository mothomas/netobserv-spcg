package pcap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	maxEventsPerPod = 1500
	maxFramesPerPod = 400
)

type PodBuffer struct {
	PodName string
	PodUID  string
	Frames  []frameRecord
	Events  []FlowEvent
	Bytes   uint64
}

// TrackedPod is a user-selected capture subject for topology filtering.
type TrackedPod struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	UID       string   `json:"uid"`
	OwnerKind string   `json:"owner_kind,omitempty"`
	PodIP     string   `json:"pod_ip,omitempty"`
	PodIPs    []string `json:"pod_ips,omitempty"`
}

type Session struct {
	ID        string
	Namespace string
	Created   time.Time
	Tracked   []TrackedPod
	pods      map[string]*PodBuffer
	s3cfg     S3CaptureConfig
	s3sink    *S3Sink
	s3info    *S3ExportInfo
	s3err     error
	mu        sync.RWMutex
}

func NewSession(namespace string) *Session {
	return &Session{
		ID:        uuid.NewString(),
		Namespace: namespace,
		Created:   time.Now().UTC(),
		pods:      make(map[string]*PodBuffer),
	}
}

// NewSessionWithS3 starts a capture that streams PCAP bytes to object storage instead of pod RAM.
func NewSessionWithS3(ctx context.Context, namespace string, cfg S3CaptureConfig) (*Session, error) {
	if err := cfg.ValidForCapture(); err != nil {
		return nil, err
	}
	s := NewSession(namespace)
	sink, err := NewS3Sink(ctx, cfg, s.ID)
	if err != nil {
		return nil, err
	}
	s.s3cfg = cfg
	s.s3sink = sink
	s.s3info = &S3ExportInfo{
		Enabled: true, Bucket: cfg.Bucket, ObjectKey: sink.ObjectKey(),
	}
	return s, nil
}

func (s *Session) S3Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s3sink != nil
}

func (s *Session) S3Config() S3CaptureConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s3cfg
}

func (s *Session) S3Export() *S3ExportInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneS3ExportInfo(s.s3info)
}

func cloneS3ExportInfo(in *S3ExportInfo) *S3ExportInfo {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func (s *Session) FinalizeS3(ctx context.Context) (*S3ExportInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.s3sink == nil {
		return cloneS3ExportInfo(s.s3info), nil
	}
	if s.s3info != nil && s.s3info.UploadDone {
		return cloneS3ExportInfo(s.s3info), nil
	}
	info, err := s.s3sink.Close(ctx)
	if err != nil {
		return nil, err
	}
	s.s3info = info
	s.s3sink = nil
	return cloneS3ExportInfo(info), nil
}

func (s *Session) LastS3Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s3err
}

func (s *Session) RefreshS3URL(ctx context.Context) (*S3ExportInfo, error) {
	s.mu.RLock()
	cfg := s.s3cfg
	info := cloneS3ExportInfo(s.s3info)
	s.mu.RUnlock()
	if info == nil || !info.Enabled || info.ObjectKey == "" {
		return nil, fmt.Errorf("s3 export not configured for session")
	}
	if !info.UploadDone {
		return info, fmt.Errorf("s3 upload still in progress")
	}
	url, err := PresignS3Object(ctx, cfg, info.ObjectKey)
	if err != nil {
		return nil, err
	}
	info.ObjectURL = url
	return info, nil
}

func (s *Session) Append(podName, podUID string, data []byte) {
	s.AppendFlow(podName, podUID, data, "", 0)
}

func (s *Session) AppendFlow(podName, podUID string, data []byte, flowMetaJSON string, seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := podUID
	if key == "" {
		key = podName
	}
	buf, ok := s.pods[key]
	if !ok {
		buf = &PodBuffer{PodName: podName, PodUID: podUID}
		s.pods[key] = buf
	}
	now := time.Now().UTC()
	if s.s3sink != nil {
		if err := s.s3sink.WriteFrame(data, now); err != nil {
			s.s3err = err
			return
		}
		buf.Events = append(buf.Events, FlowEvent{
			At: now, CapturePod: podName, CapturePodUID: podUID,
			FlowMeta: parseFlowMeta(flowMetaJSON), Sequence: seq,
		})
		trimEventsOnly(buf)
		buf.Bytes += uint64(len(data))
		if s.s3info != nil {
			s.s3info.Bytes = s.s3sink.BytesUploaded()
		}
		return
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	buf.Frames = append(buf.Frames, frameRecord{Data: cp, At: now})
	buf.Events = append(buf.Events, FlowEvent{
		At: now, CapturePod: podName, CapturePodUID: podUID,
		Frame: cp, FlowMeta: parseFlowMeta(flowMetaJSON), Sequence: seq,
	})
	trimPodBuffer(buf)
	buf.Bytes += uint64(len(data))
}

func trimEventsOnly(buf *PodBuffer) {
	if len(buf.Events) > maxEventsPerPod {
		buf.Events = buf.Events[len(buf.Events)-maxEventsPerPod:]
	}
}

func (s *Session) SetTrackedPods(pods []TrackedPod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tracked = append([]TrackedPod(nil), pods...)
}

func (s *Session) TrackedPods() []TrackedPod {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]TrackedPod(nil), s.Tracked...)
}

func (s *Session) TrackedNodeIDs() map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]struct{}, len(s.Tracked))
	for _, p := range s.Tracked {
		if p.Namespace != "" && p.Name != "" {
			out[p.Namespace+"/"+p.Name] = struct{}{}
		}
	}
	return out
}

// TrackedPodIDList returns ns/name keys for UI tenant scoping.
func (s *Session) TrackedPodIDList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.Tracked))
	for _, p := range s.Tracked {
		if p.Namespace != "" && p.Name != "" {
			out = append(out, p.Namespace+"/"+p.Name)
		}
	}
	return out
}

func trimPodBuffer(buf *PodBuffer) {
	if len(buf.Events) > maxEventsPerPod {
		buf.Events = buf.Events[len(buf.Events)-maxEventsPerPod:]
	}
	if len(buf.Frames) > maxFramesPerPod {
		buf.Frames = buf.Frames[len(buf.Frames)-maxFramesPerPod:]
	}
}

func (s *Session) TotalBytes() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n uint64
	for _, p := range s.pods {
		n += p.Bytes
	}
	return n
}

func (s *Session) Events() []FlowEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FlowEvent
	for _, p := range s.pods {
		out = append(out, p.Events...)
	}
	return out
}

func (s *Session) PodNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.pods))
	for _, p := range s.pods {
		out = append(out, p.PodName)
	}
	return out
}

func (s *Session) ExportPod(podUID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.s3sink != nil || (s.s3info != nil && s.s3info.Enabled) {
		return nil, fmt.Errorf("per-pod PCAP export unavailable when streaming to S3; use merged object")
	}
	buf, ok := s.pods[podUID]
	if !ok {
		return nil, fmt.Errorf("no capture buffer for pod uid %s", podUID)
	}
	return encodeFrames(buf.Frames), nil
}

// PodExportName returns a filesystem-safe pod name for PCAP downloads.
func (s *Session) PodExportName(podUID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if buf, ok := s.pods[podUID]; ok && buf.PodName != "" {
		return buf.PodName
	}
	for _, t := range s.Tracked {
		if t.UID == podUID && t.Name != "" {
			return t.Name
		}
	}
	return podUID
}

func (s *Session) ExportMerged() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.s3sink != nil || (s.s3info != nil && s.s3info.Enabled) {
		return nil, fmt.Errorf("merged PCAP is stored in S3 for this session")
	}
	var all []frameRecord
	for _, p := range s.pods {
		all = append(all, p.Frames...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("session has no captured data to merge")
	}
	return encodeFrames(all), nil
}

func encodeFrames(frames []frameRecord) []byte {
	if len(frames) == 0 {
		return nil
	}
	raw := concatFrameBytes(frames)
	if IsPCAPContainer(raw) {
		return raw
	}
	return EncodePCAPng(frames)
}

func concatFrameBytes(frames []frameRecord) []byte {
	var n int
	for _, f := range frames {
		n += len(f.Data)
	}
	out := make([]byte, 0, n)
	for _, f := range frames {
		out = append(out, f.Data...)
	}
	return out
}

var globalSessions sync.Map

func Store(sess *Session) {
	globalSessions.Store(sess.ID, sess)
}

func Get(id string) (*Session, bool) {
	v, ok := globalSessions.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

func Delete(id string) {
	globalSessions.Delete(id)
}
