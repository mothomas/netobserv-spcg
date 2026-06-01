package sensor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

// Target describes a user-selected capture subject (pod-level and/or owner-level).
type Target struct {
	SessionID     string
	Namespace     string
	PodName       string
	PodUID        string
	WorkloadKind  string
	WorkloadName  string
	LabelSelector string
	Port          int32
}

// Manager deploys netobserv eBPF sensors (DaemonSet) in the capture namespace.
type Manager struct {
	Client            kubernetes.Interface
	CaptureNamespace  string
	CollectorHost     string
	AgentImage        string
}

func NewManager(client kubernetes.Interface) *Manager {
	ns := envOr("CAPTURE_NAMESPACE", "pcap-capture")
	host := envOr("COLLECTOR_HOST", "")
	if host == "" {
		host = envOr("POD_IP", "spcg-backend-engine."+ns+".svc.cluster.local")
	}
	return &Manager{
		Client:           client,
		CaptureNamespace: ns,
		CollectorHost:    host,
		AgentImage:       envOr("NETOBSERV_AGENT_IMAGE", "quay.io/netobserv/netobserv-ebpf-agent:release-1.8"),
	}
}

type Session struct {
	ID           string
	Port         int
	DaemonSet    string
	Collector    *PacketCollector
	TrackedPods  []spcgk8s.PodDetail
	CancelDeploy context.CancelFunc
	RefreshCh    chan PodRefreshEvent
}

func (m *Manager) StartSession(ctx context.Context, sessionID string, port int, targets []Target) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one capture target is required")
	}

	collector, err := StartPacketCollector(port)
	if err != nil {
		return nil, fmt.Errorf("failed starting netobserv packet collector: %w", err)
	}

	dsName := daemonSetName(sessionID)
	resolvedPods, err := m.resolveTargetPods(ctx, targets)
	if err != nil {
		collector.Close()
		return nil, fmt.Errorf("failed resolving capture pods: %w", err)
	}
	manifest, err := m.renderDaemonSet(sessionID, dsName, port, targets, resolvedPods)
	if err != nil {
		collector.Close()
		return nil, fmt.Errorf("failed rendering netobserv sensor manifest: %w", err)
	}

	deployCtx, cancel := context.WithCancel(ctx)
	if err := m.applyDaemonSet(deployCtx, manifest); err != nil {
		cancel()
		collector.Close()
		return nil, fmt.Errorf("failed deploying netobserv eBPF sensor: %w", err)
	}
	log.Printf("spcg-sensor: deployed daemonset %s/%s on collector port %d", m.CaptureNamespace, dsName, port)

	if err := m.waitDaemonSetReady(deployCtx, dsName, 3*time.Minute); err != nil {
		cancel()
		_ = m.deleteDaemonSet(context.Background(), dsName)
		collector.Close()
		return nil, fmt.Errorf("failed waiting for netobserv sensor readiness: %w", err)
	}

	sess := &Session{
		ID: sessionID, Port: port, DaemonSet: dsName,
		Collector: collector, TrackedPods: resolvedPods, CancelDeploy: cancel,
		RefreshCh: make(chan PodRefreshEvent, 2),
	}
	go m.watchPods(deployCtx, sess, targets, podsFingerprint(resolvedPods))
	return sess, nil
}

func (m *Manager) StopSession(ctx context.Context, sess *Session) error {
	if sess == nil {
		return nil
	}
	if sess.CancelDeploy != nil {
		sess.CancelDeploy()
	}
	if sess.Collector != nil {
		sess.Collector.Close()
	}
	if sess.DaemonSet != "" {
		if err := m.deleteDaemonSet(ctx, sess.DaemonSet); err != nil {
			return fmt.Errorf("failed deleting netobserv sensor daemonset: %w", err)
		}
	}
	return nil
}

func (m *Manager) renderDaemonSet(sessionID, dsName string, port int, targets []Target, resolvedPods []spcgk8s.PodDetail) (string, error) {
	filters := buildEBPFFilterRules(resolvedPods)
	tmpl, err := template.New("ds").Parse(packetCaptureDaemonSetTemplate)
	if err != nil {
		return "", fmt.Errorf("failed parsing daemonset template: %w", err)
	}
	var buf bytes.Buffer
	buildID := envOr("SPCG_BUILD_ID", "dev")
	log.Printf("spcg-sensor: render ds=%s build=%s export=grpc target=%s:%d", dsName, buildID, m.CollectorHost, port)
	err = tmpl.Execute(&buf, map[string]string{
		"DAEMONSET_NAME":    dsName,
		"CAPTURE_NAMESPACE": m.CaptureNamespace,
		"SESSION_ID":        sessionID,
		"AGENT_IMAGE":       m.AgentImage,
		"BUILD_ID":          buildID,
		"COLLECTOR_HOST":     m.CollectorHost,
		"COLLECTOR_PORT":     fmt.Sprintf("%d", port),
		"FLOW_FILTER_RULES":  filters,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (m *Manager) buildFLPConfig(port int, _ []Target, _ []spcgk8s.PodDetail) (string, error) {
	cfg := strings.ReplaceAll(collectorPipelineConfigJSON, "{{TARGET_HOST}}", m.CollectorHost)
	cfg = strings.ReplaceAll(cfg, "{{TARGET_PORT}}", fmt.Sprintf("%d", port))
	return compactJSONString(cfg)
}

func compactJSONString(s string) (string, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildEBPFFilterRules(pods []spcgk8s.PodDetail) string {
	type flowRule struct {
		IPCidr   string `json:"ip_cidr"`
		PeerCidr string `json:"peer_cidr,omitempty"`
		Action   string `json:"action"`
	}
	rules := make([]flowRule, 0, len(pods)*2)
	seen := map[string]struct{}{}
	for _, p := range pods {
		ips := p.PodIPs
		if len(ips) == 0 && p.PodIP != "" {
			ips = []string{p.PodIP}
		}
		for _, ip := range ips {
			if ip == "" {
				continue
			}
			cidr := podIPCidr(ip)
			if _, ok := seen[cidr]; ok {
				continue
			}
			seen[cidr] = struct{}{}
			peer := "0.0.0.0/0"
			if strings.Contains(ip, ":") {
				peer = "::/0"
			}
			rules = append(rules, flowRule{IPCidr: cidr, PeerCidr: peer, Action: "Accept"})
		}
	}
	if len(rules) == 0 {
		return `[{"ip_cidr":"0.0.0.0/0","action":"Accept"}]`
	}
	b, err := json.Marshal(rules)
	if err != nil {
		return `[{"ip_cidr":"0.0.0.0/0","action":"Accept"}]`
	}
	return string(b)
}

func podIPCidr(ip string) string {
	if strings.Contains(ip, ":") {
		return ip + "/128"
	}
	return ip + "/32"
}

func (m *Manager) resolveTargetPods(ctx context.Context, targets []Target) ([]spcgk8s.PodDetail, error) {
	selections := make([]spcgk8s.CaptureSelection, 0, len(targets))
	for _, t := range targets {
		if t.PodName != "" {
			selections = append(selections, spcgk8s.CaptureSelection{
				Type: "pod", Namespace: t.Namespace, PodName: t.PodName, PodUID: t.PodUID, Port: t.Port,
			})
			continue
		}
		if t.WorkloadKind != "" && t.WorkloadName != "" {
			selections = append(selections, spcgk8s.CaptureSelection{
				Type: "owner", Namespace: t.Namespace,
				OwnerKind: t.WorkloadKind, OwnerName: t.WorkloadName,
				LabelSelector: t.LabelSelector, Port: t.Port,
			})
		}
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("no resolvable capture targets")
	}
	resolved, err := spcgk8s.ResolveCaptureSelections(ctx, m.Client, selections)
	if err != nil {
		return nil, err
	}
	log.Printf("spcg-sensor: resolved %d pods from %d targets", len(resolved.Pods), len(targets))
	return resolved.Pods, nil
}

func (m *Manager) applyDaemonSet(ctx context.Context, manifest string) error {
	obj := &appsv1.DaemonSet{}
	if err := yaml.Unmarshal([]byte(manifest), obj); err != nil {
		return fmt.Errorf("failed decoding daemonset yaml: %w", err)
	}
	_, err := m.Client.AppsV1().DaemonSets(m.CaptureNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) deleteDaemonSet(ctx context.Context, name string) error {
	return m.Client.AppsV1().DaemonSets(m.CaptureNamespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (m *Manager) waitDaemonSetReady(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ds, err := m.Client.AppsV1().DaemonSets(m.CaptureNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for daemonset %s/%s", m.CaptureNamespace, name)
}

func daemonSetName(sessionID string) string {
	s := strings.ReplaceAll(sessionID, "-", "")
	if len(s) > 12 {
		s = s[:12]
	}
	return "spcg-sensor-" + s
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
