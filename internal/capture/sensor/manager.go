package sensor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

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
	CancelDeploy context.CancelFunc
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
	manifest, err := m.renderDaemonSet(sessionID, dsName, port, targets)
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

	if err := m.waitDaemonSetReady(deployCtx, dsName, 3*time.Minute); err != nil {
		cancel()
		_ = m.deleteDaemonSet(context.Background(), dsName)
		collector.Close()
		return nil, fmt.Errorf("failed waiting for netobserv sensor readiness: %w", err)
	}

	return &Session{
		ID: sessionID, Port: port, DaemonSet: dsName,
		Collector: collector, CancelDeploy: cancel,
	}, nil
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

func (m *Manager) renderDaemonSet(sessionID, dsName string, port int, targets []Target) (string, error) {
	flp, err := m.buildFLPConfig(port, targets)
	if err != nil {
		return "", err
	}
	filters := buildFlowFilterRules(targets)

	tmpl, err := template.New("ds").Parse(packetCaptureDaemonSetTemplate)
	if err != nil {
		return "", fmt.Errorf("failed parsing daemonset template: %w", err)
	}
	var buf bytes.Buffer
	flp = strings.ReplaceAll(flp, "'", "''")
	err = tmpl.Execute(&buf, map[string]string{
		"DAEMONSET_NAME":    dsName,
		"CAPTURE_NAMESPACE": m.CaptureNamespace,
		"SESSION_ID":        sessionID,
		"AGENT_IMAGE":       m.AgentImage,
		"FLOW_FILTER_RULES": filters,
		"FLP_CONFIG":        flp,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (m *Manager) buildFLPConfig(port int, targets []Target) (string, error) {
	cfg := strings.ReplaceAll(collectorPipelineConfigJSON, "{{TARGET_HOST}}", m.CollectorHost)
	cfg = strings.ReplaceAll(cfg, "{{TARGET_PORT}}", fmt.Sprintf("%d", port))

	// Optional keep_entry_query filter in FLP (netobserv-cli pattern).
	query := buildKeepEntryQuery(targets)
	if query == "" {
		return compactJSONString(cfg)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(cfg), &doc); err != nil {
		return "", fmt.Errorf("failed parsing collector pipeline config: %w", err)
	}
	rule := map[string]interface{}{
		"type":            "keep_entry_query",
		"keepEntryQuery": query,
	}
	params, _ := doc["parameters"].([]interface{})
	filterParam := map[string]interface{}{
		"name": "filter",
		"transform": map[string]interface{}{
			"type":   "filter",
			"filter": map[string]interface{}{"rules": []interface{}{rule}},
		},
	}
	params = append(params, filterParam)
	doc["parameters"] = params
	pipe, _ := doc["pipeline"].([]interface{})
	pipe = append(pipe,
		map[string]interface{}{"name": "filter", "follows": "enrich"},
		map[string]interface{}{"name": "send", "follows": "filter"},
	)
	// remove duplicate send follows enrich
	filtered := make([]interface{}, 0, len(pipe))
	seen := map[string]bool{}
	for _, p := range pipe {
		m, _ := p.(map[string]interface{})
		name, _ := m["name"].(string)
		if name == "send" && m["follows"] == "enrich" {
			continue
		}
		key := name + fmt.Sprint(m["follows"])
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, p)
	}
	doc["pipeline"] = filtered

	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
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

func buildFlowFilterRules(targets []Target) string {
	base := strings.TrimSpace(flowFilterTemplateJSON)
	_ = targets
	return base
}

func buildKeepEntryQuery(targets []Target) string {
	var clauses []string
	seen := map[string]struct{}{}
	for _, t := range targets {
		if t.Namespace == "" {
			continue
		}
		var clause string
		if t.WorkloadKind != "" && t.WorkloadName != "" {
			clause = fmt.Sprintf(
				`(SrcK8S_Namespace=="%s" && SrcK8S_OwnerType=="%s" && SrcK8S_OwnerName=="%s") || (DstK8S_Namespace=="%s" && DstK8S_OwnerType=="%s" && DstK8S_OwnerName=="%s")`,
				t.Namespace, t.WorkloadKind, t.WorkloadName,
				t.Namespace, t.WorkloadKind, t.WorkloadName,
			)
		} else if t.PodName != "" {
			clause = fmt.Sprintf(
				`(SrcK8S_Namespace=="%s" && SrcK8S_Name=="%s") || (DstK8S_Namespace=="%s" && DstK8S_Name=="%s")`,
				t.Namespace, t.PodName, t.Namespace, t.PodName,
			)
		} else {
			continue
		}
		if _, ok := seen[clause]; ok {
			continue
		}
		seen[clause] = struct{}{}
		clauses = append(clauses, "("+clause+")")
	}
	return strings.Join(clauses, " || ")
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
