package sensor

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"time"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

// PodRefreshEvent signals capture targets changed (pod restart / reschedule).
type PodRefreshEvent struct {
	Pods []spcgk8s.PodDetail `json:"pods"`
}

// watchPods periodically re-resolves targets and notifies the capture stream (no eBPF filter churn).
func (m *Manager) watchPods(ctx context.Context, sess *Session, targets []Target, initialKey string) {
	if sess == nil || sess.RefreshCh == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	lastKey := initialKey
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := m.resolveTargetPods(ctx, targets)
			if err != nil {
				log.Printf("spcg-sensor: pod watch resolve failed session=%s: %v", sess.ID, err)
				continue
			}
			key := podsFingerprint(pods)
			if key == lastKey {
				continue
			}
			lastKey = key
			sess.TrackedPods = pods
			log.Printf("spcg-sensor: tracked pods updated session=%s pods=%d", sess.ID, len(pods))
			select {
			case sess.RefreshCh <- PodRefreshEvent{Pods: pods}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

func podsFingerprint(pods []spcgk8s.PodDetail) string {
	type row struct {
		UID string `json:"u"`
		IP  string `json:"i"`
	}
	rows := make([]row, 0, len(pods))
	for _, p := range pods {
		rows = append(rows, row{UID: p.UID, IP: p.PodIP})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].UID != rows[j].UID {
			return rows[i].UID < rows[j].UID
		}
		return rows[i].IP < rows[j].IP
	})
	b, _ := json.Marshal(rows)
	return string(b)
}
