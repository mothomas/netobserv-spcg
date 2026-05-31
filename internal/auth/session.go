package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const HeaderSPCGSession = "X-SPCG-Session"

type Mode string

const (
	ModeBearer     Mode = "bearer"
	ModeKubeconfig Mode = "kubeconfig"
)

type Session struct {
	ID         string
	Mode       Mode
	Bearer     string
	Kubeconfig []byte
	Created    time.Time
}

type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Store{sessions: make(map[string]*Session), ttl: ttl}
}

func (s *Store) CreateBearer(token string) (*Session, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}
	sess := &Session{
		ID:      newSessionID(),
		Mode:    ModeBearer,
		Bearer:  token,
		Created: time.Now(),
	}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	return sess, nil
}

func (s *Store) CreateKubeconfig(content []byte) (*Session, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("kubeconfig content is required")
	}
	cp := make([]byte, len(content))
	copy(cp, content)
	sess := &Session{
		ID:         newSessionID(),
		Mode:       ModeKubeconfig,
		Kubeconfig: cp,
		Created:    time.Now(),
	}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	return sess, nil
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	if time.Since(sess.Created) > s.ttl {
		s.wipeLocked(id)
		return nil, false
	}
	return sess, true
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wipeLocked(id)
}

func (s *Store) wipeLocked(id string) {
	sess, ok := s.sessions[id]
	if !ok {
		return
	}
	Wipe([]byte(sess.Bearer))
	Wipe(sess.Kubeconfig)
	delete(s.sessions, id)
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
