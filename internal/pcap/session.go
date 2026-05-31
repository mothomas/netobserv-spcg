package pcap

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type PodBuffer struct {
	PodName string
	PodUID  string
	Chunks  [][]byte
	Bytes   uint64
}

type Session struct {
	ID        string
	Namespace string
	Created   time.Time
	pods      map[string]*PodBuffer
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

func (s *Session) Append(podName, podUID string, data []byte) {
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
	cp := make([]byte, len(data))
	copy(cp, data)
	buf.Chunks = append(buf.Chunks, cp)
	buf.Bytes += uint64(len(data))
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
	buf, ok := s.pods[podUID]
	if !ok {
		return nil, fmt.Errorf("no capture buffer for pod uid %s", podUID)
	}
	return concatChunks(buf.Chunks), nil
}

func (s *Session) ExportMerged() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all [][]byte
	for _, p := range s.pods {
		all = append(all, p.Chunks...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("session has no captured data to merge")
	}
	return concatChunks(all), nil
}

func concatChunks(chunks [][]byte) []byte {
	var n int
	for _, c := range chunks {
		n += len(c)
	}
	out := make([]byte, 0, n)
	for _, c := range chunks {
		out = append(out, c...)
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
