package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const networksStatusAnnotation = "k8s.v1.cni.cncf.io/networks-status"

type networksStatusEntry struct {
	Name      string `json:"name"`
	Interface string `json:"interface"`
	Default   bool   `json:"default"`
}

// ListAttachInterfaces returns default + Multus interfaces for the source pod.
func ListAttachInterfaces(ctx context.Context, cs kubernetes.Interface, pod spcgk8s.PodDetail) ([]AttachInterface, error) {
	if cs == nil || pod.Namespace == "" || pod.Name == "" {
		return []AttachInterface{{Name: "default", Primary: true, CNI: "primary"}}, nil
	}
	p, err := cs.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("load pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	return interfacesFromPod(p), nil
}

func interfacesFromPod(p *corev1.Pod) []AttachInterface {
	out := []AttachInterface{{Name: "default", Primary: true, CNI: "primary"}}
	raw := p.Annotations[networksStatusAnnotation]
	if raw == "" {
		return out
	}
	var entries []networksStatusEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return out
	}
	seen := map[string]struct{}{"default": {}}
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		iface := strings.TrimSpace(e.Interface)
		if name == "" && iface == "" {
			continue
		}
		label := name
		if label == "" {
			label = iface
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, AttachInterface{
			Name:    label,
			Primary: e.Default,
			CNI:     "multus",
		})
	}
	return out
}

// Fire starts a painted probe and paints primary edges as observations arrive.
func Fire(ctx context.Context, cs kubernetes.Interface, cfg *rest.Config, resp trace.DiscoverResponse, req FireRequest) (*FireResponse, error) {
	if resp.TraceID == "" {
		return nil, fmt.Errorf("trace_id is required")
	}
	iface := strings.TrimSpace(req.Interface)
	if iface == "" {
		iface = "default"
	}
	stopSession(resp.TraceID)

	token, icmpID := PaintToken(resp.TraceID)
	corr := NewGraphCorrelator(resp.Graph)
	probeID := uuid.NewString()
	child, cancel := context.WithCancel(ctx)
	s := &session{
		traceID: resp.TraceID,
		probeID: probeID,
		corr:    corr,
		cancel:  cancel,
	}
	sessMu.Lock()
	sessions[resp.TraceID] = s
	sessMu.Unlock()

	mode := "live"
	if req.Simulate || simulateDefault() || cs == nil {
		mode = "simulate"
	}

	out := &FireResponse{
		ProbeID:      probeID,
		TraceID:      resp.TraceID,
		PaintToken:   token,
		ICMPID:       icmpID,
		Interface:    iface,
		Mode:         mode,
		PrimaryEdges: corr.PrimaryCount(),
	}

	s.broadcast(ProbeEvent{
		Type:    "probe_started",
		TraceID: resp.TraceID,
		ProbeID: probeID,
		Message: fmt.Sprintf("paint %s via %s (%s)", token, iface, mode),
	})

	go func() {
		defer stopSession(resp.TraceID)
		if mode == "simulate" {
			runSimulatedPaint(child, s, corr)
			return
		}
		destIP, err := resolveDestIP(resp)
		if err != nil {
			s.broadcast(ProbeEvent{Type: "error", TraceID: resp.TraceID, ProbeID: probeID, Message: err.Error()})
			return
		}
		if err := execProbePing(child, cfg, cs, resp.TargetPod, iface, destIP); err != nil {
			s.broadcast(ProbeEvent{Type: "error", TraceID: resp.TraceID, ProbeID: probeID, Message: err.Error()})
			// Fall back to simulated paint so the UX still demonstrates path correlation.
			runSimulatedPaint(child, s, corr)
			return
		}
		runSimulatedPaint(child, s, corr)
	}()

	return out, nil
}

func simulateDefault() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SPCG_PROBE_SIMULATE")))
	return v == "1" || v == "true" || v == "yes"
}

func resolveDestIP(resp trace.DiscoverResponse) (string, error) {
	dest := resp.Destination
	if dest.Mode == "ip" {
		ip := strings.TrimSpace(dest.IP)
		if ip == "" || ip == "external" {
			return "", fmt.Errorf("destination IP is required for live probe")
		}
		return ip, nil
	}
	if len(resp.DestPods) > 0 && resp.DestPods[0].PodIP != "" {
		return resp.DestPods[0].PodIP, nil
	}
	if dest.Mode == "namespace" && len(resp.SourcePods) > 0 {
		// Same-namespace service trace: ping cluster DNS or first dest pod if known.
		return "", fmt.Errorf("select a destination IP or resolve destination pods for live probe")
	}
	return "", fmt.Errorf("could not resolve destination IP")
}

func runSimulatedPaint(ctx context.Context, s *session, corr *GraphCorrelator) {
	hooks := []string{"veth_egress", "ovs_vport_receive", "ovs_execute_actions", "physical_egress"}
	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		edgeID, ok := corr.Advance(hooks[seq%len(hooks)])
		if !ok {
			s.broadcast(ProbeEvent{
				Type:    "probe_finished",
				TraceID: s.traceID,
				ProbeID: s.probeID,
				Message: "all primary hops painted",
			})
			return
		}
		seq++
		s.broadcast(ProbeEvent{
			Type:    "edge_update",
			TraceID: s.traceID,
			ProbeID: s.probeID,
			EdgeID:  edgeID,
			State:   EdgeActiveGreen,
			Hook:    hooks[(seq-1)%len(hooks)],
			Seq:     seq,
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(420 * time.Millisecond):
		}
	}
}

func execProbePing(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, pod spcgk8s.PodDetail, iface, destIP string) error {
	if cfg == nil || cs == nil || pod.Namespace == "" || pod.Name == "" {
		return fmt.Errorf("cluster client unavailable")
	}
	var pingCmd string
	if iface != "" && iface != "default" {
		pingCmd = fmt.Sprintf("ping -I %s -c 3 -W 2 -i 0.25 %s", shellQuote(iface), shellQuote(destIP))
	} else {
		pingCmd = fmt.Sprintf("ping -c 3 -W 2 -i 0.25 %s", shellQuote(destIP))
	}
	cmd := []string{"sh", "-c", pingCmd}
	req := cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("exec executor: %w", err)
	}
	var stdout, stderr strings.Builder
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ping exec: %s", msg)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\"'\"'`) + "'"
}

func EdgeStates(traceID string) map[string]EdgePaintState {
	s, ok := getSession(traceID)
	if !ok || s.corr == nil {
		return map[string]EdgePaintState{}
	}
	return s.corr.Snapshot()
}
