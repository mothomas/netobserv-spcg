package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	capturev1 "github.com/netobserv/spcg/api/proto/capture/v1"
	"github.com/netobserv/spcg/internal/ai"
	"github.com/netobserv/spcg/internal/auth"
	"github.com/netobserv/spcg/internal/capture/sensor"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/pcap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"google.golang.org/grpc"
	grpcreds "google.golang.org/grpc/credentials"
)

type Server struct {
	EngineAddr string
	EngineTLS  grpcreds.TransportCredentials
	Sessions   *auth.Store
}

type aiSessionCreds struct {
	proxyURL       string
	targetType     ai.TargetType
	apiEndpoint    string
	bearer         []byte
	cursorAgentID  string
}

var (
	aiSessionsMu sync.Mutex
	aiSessions   = make(map[string]*aiSessionCreds)
)

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/v1/namespaces", s.handleNamespaces)
	mux.HandleFunc("/api/v1/namespaces/", s.handleNamespaceSubresource)
	mux.HandleFunc("/api/v1/workloads", s.handleWorkloads)
	mux.HandleFunc("/api/v1/capture/stream", s.handleCaptureStream)
	mux.HandleFunc("/api/v1/capture/", s.handleCaptureDownload)
	s.registerAIRoutes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) userClient(r *http.Request) (*spcgk8s.ClientsetWrap, error) {
	if s.Sessions == nil {
		s.Sessions = auth.NewStore(0)
	}
	sid, mode, bearer, err := auth.ResolveSessionID(r)
	if err != nil {
		return nil, err
	}
	if sid != "" {
		sess, ok := s.Sessions.Get(sid)
		if !ok {
			return nil, fmt.Errorf("session expired or invalid: re-authenticate")
		}
		switch sess.Mode {
		case auth.ModeKubeconfig:
			cs, _, err := spcgk8s.ClientsetFromKubeconfig(sess.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed building client from session kubeconfig: %w", err)
			}
			return &spcgk8s.ClientsetWrap{Interface: cs}, nil
		case auth.ModeBearer:
			cs, err := spcgk8s.ClientsetFromBearerToken(sess.Bearer)
			if err != nil {
				return nil, err
			}
			return &spcgk8s.ClientsetWrap{Interface: cs}, nil
		default:
			return nil, fmt.Errorf("unsupported session mode")
		}
	}
	if mode == auth.ModeBearer && bearer != "" {
		cs, err := spcgk8s.ClientsetFromBearerToken(bearer)
		if err != nil {
			return nil, err
		}
		return &spcgk8s.ClientsetWrap{Interface: cs}, nil
	}
	return nil, fmt.Errorf("missing authentication: login or send Authorization / %s", auth.HeaderSPCGSession)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Sessions == nil {
		s.Sessions = auth.NewStore(0)
	}
	var body struct {
		Mode       string `json:"mode"`
		Token      string `json:"token"`
		Kubeconfig string `json:"kubeconfig"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var sess *auth.Session
	var clusterHost string

	switch strings.ToLower(body.Mode) {
	case "kubeconfig", "kube", "config":
		kc, err := auth.DecodeKubeconfigUpload(body.Kubeconfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cs, cfg, err := spcgk8s.ClientsetFromKubeconfig(kc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := cs.CoreV1().Namespaces().List(r.Context(), metav1.ListOptions{Limit: 1}); err != nil {
			http.Error(w, fmt.Sprintf("kubeconfig rejected by API server: %v", err), http.StatusUnauthorized)
			return
		}
		clusterHost = spcgk8s.ClusterHost(cfg)
		sess, err = s.Sessions.CreateKubeconfig(kc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		auth.Wipe(kc)
	default:
		if body.Token == "" {
			http.Error(w, "token is required for bearer mode", http.StatusBadRequest)
			return
		}
		cs, err := spcgk8s.ClientsetFromBearerToken(body.Token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := cs.CoreV1().Namespaces().List(r.Context(), metav1.ListOptions{Limit: 1}); err != nil {
			http.Error(w, fmt.Sprintf("token rejected by API server: %v", err), http.StatusUnauthorized)
			return
		}
		sess, err = s.Sessions.CreateBearer(body.Token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, map[string]string{
		"session_id": sess.ID,
		"mode":       string(sess.Mode),
		"cluster":    clusterHost,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Sessions == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sid := strings.TrimSpace(r.Header.Get(auth.HeaderSPCGSession))
	if sid != "" {
		s.Sessions.Delete(sid)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cs, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	list, err := cs.CoreV1().Namespaces().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed listing namespaces: %v", err), http.StatusForbidden)
		return
	}
	type nsRow struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	out := make([]nsRow, 0, len(list.Items))
	for _, n := range list.Items {
		out = append(out, nsRow{Name: n.Name, Status: string(n.Status.Phase)})
	}
	writeJSON(w, out)
}

func (s *Server) handleNamespaceSubresource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "expected /api/v1/namespaces/{ns}/workloads", http.StatusBadRequest)
		return
	}
	ns, resource := parts[0], parts[1]
	if resource != "workloads" || r.Method != http.MethodGet {
		http.Error(w, "unsupported subresource", http.StatusNotFound)
		return
	}
	cs, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	wl, err := spcgk8s.ListNamespaceWorkloads(r.Context(), cs, ns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, wl)
}

func (s *Server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cs, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	var body struct {
		Namespaces []string `json:"namespaces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(body.Namespaces) == 0 {
		http.Error(w, "namespaces array is required", http.StatusBadRequest)
		return
	}
	out, err := spcgk8s.ListWorkloadsAcrossNamespaces(r.Context(), cs, body.Namespaces)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

type captureStartRequest struct {
	Namespaces  []string                    `json:"namespaces"`
	Namespace   string                      `json:"namespace"`
	Selections  []spcgk8s.CaptureSelection  `json:"selections"`
}

func (s *Server) handleCaptureStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req captureStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Selections) == 0 {
		http.Error(w, "selections array is required", http.StatusBadRequest)
		return
	}

	cs, err := s.userClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resolved, err := spcgk8s.ResolveCaptureSelections(r.Context(), cs, req.Selections)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sessNS := req.Namespace
	if sessNS == "" && len(req.Namespaces) > 0 {
		sessNS = req.Namespaces[0]
	}
	sess := pcap.NewSession(sessNS)
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

	meta, _ := json.Marshal(map[string]interface{}{
		"session_id": sess.ID, "resolved_pods": len(resolved.Pods), "sensor_filters": len(resolved.SensorTargets),
	})
	fmt.Fprintf(w, "event: session\ndata: %s\n\n", meta)
	flusher.Flush()

	ctx := r.Context()
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(s.EngineTLS)}
	conn, err := grpc.NewClient(s.EngineAddr, dialOpts...)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer conn.Close()

	client := capturev1.NewCaptureServiceClient(conn)
	stream, err := client.StreamPackets(ctx)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
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
			fmt.Fprintf(w, "event: error\ndata: send failed: %v\n\n", err)
			flusher.Flush()
			return
		}
	}
	if err := stream.CloseSend(); err != nil {
		fmt.Fprintf(w, "event: error\ndata: close send failed: %v\n\n", err)
		flusher.Flush()
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %v\n\n", err)
				flusher.Flush()
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
					payload, _ := json.Marshal(map[string]interface{}{
						"session_id": sess.ID, "pods": refresh.Pods, "stitched": true,
					})
					fmt.Fprintf(w, "event: pod_refresh\ndata: %s\n\n", payload)
					flusher.Flush()
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
			chunkPayload := map[string]interface{}{
				"session_id":       chunk.GetSessionId(),
				"pod_name":         podName,
				"sequence":         chunk.GetSequence(),
				"chunk_size":       len(chunk.GetData()),
				"packets_per_sec":  chunk.GetPacketsPerSec(),
				"cumulative_bytes": chunk.GetCumulativeBytes(),
				"stitched_restart": chunk.GetStitchedRestart(),
			}
			if meta != "" {
				chunkPayload["flow_metadata"] = json.RawMessage(meta)
			}
			payload, _ := json.Marshal(chunkPayload)
			fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}()

	<-r.Context().Done()
	<-done
	fmt.Fprintf(w, "event: end\ndata: %s\n\n", sess.ID)
	flusher.Flush()
}

func (s *Server) handleCaptureDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/capture/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "expected .../download/{session} or .../merge/{session}", http.StatusBadRequest)
		return
	}
	action, sessionID := parts[0], parts[1]
	sess, ok := pcap.Get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	var data []byte
	var err error
	var filename string

	switch action {
	case "download":
		podUID := r.URL.Query().Get("pod_uid")
		data, err = sess.ExportPod(podUID)
		filename = fmt.Sprintf("%s-%s.pcapng", sess.Namespace, podUID)
	case "merge":
		data, err = sess.ExportMerged()
		filename = fmt.Sprintf("%s-merged.pcapng", sess.Namespace)
	default:
		http.Error(w, "unknown capture action", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcapng")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = io.Copy(w, bytes.NewReader(data))
}

func (s *Server) handleAITriage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		SessionID   string `json:"session_id"`
		TraceText   string `json:"trace_text"`
		ProxyURL    string `json:"proxy_url"`
		TargetType  string `json:"target_type"`
		APIEndpoint string `json:"api_endpoint"`
		BearerToken string `json:"bearer_token"`
		Flush       bool   `json:"flush_session"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if body.Flush {
		aiSessionsMu.Lock()
		if c, ok := aiSessions[body.SessionID]; ok {
			auth.Wipe(c.bearer)
			delete(aiSessions, body.SessionID)
		}
		aiSessionsMu.Unlock()
		ai.ResetMaps()
		ai.DropScrubber(body.SessionID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	trace := body.TraceText
	if trace == "" && body.SessionID != "" {
		if sess, ok := pcap.Get(body.SessionID); ok {
			b, err := sess.ExportMerged()
			if err == nil {
				trace = hexPreview(b, 65536)
			}
		}
	}

	creds := []byte(body.BearerToken)
	aiSessionsMu.Lock()
	aiSessions[body.SessionID] = &aiSessionCreds{
		proxyURL: body.ProxyURL, targetType: ai.TargetType(body.TargetType),
		apiEndpoint: body.APIEndpoint, bearer: creds,
	}
	aiSessionsMu.Unlock()

	client := ai.NewTriageClient(body.ProxyURL)
	resp, err := client.Run(r.Context(), ai.TriageRequest{
		TraceText: trace, ProxyURL: body.ProxyURL,
		TargetType: ai.TargetType(body.TargetType), APIEndpoint: body.APIEndpoint,
		BearerToken: body.BearerToken,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, resp)
}

func hexPreview(b []byte, max int) string {
	if len(b) > max {
		b = b[:max]
	}
	const hex = "0123456789abcdef"
	var sb strings.Builder
	for i, by := range b {
		if i > 0 && i%16 == 0 {
			sb.WriteByte('\n')
		}
		sb.WriteByte(hex[by>>4])
		sb.WriteByte(hex[by&0x0f])
		sb.WriteByte(' ')
	}
	return sb.String()
}

func trackedPodsFromSession(sess *pcap.Session) []spcgk8s.PodDetail {
	tracked := sess.TrackedPods()
	out := make([]spcgk8s.PodDetail, 0, len(tracked))
	for _, t := range tracked {
		out = append(out, spcgk8s.PodDetail{
			Namespace: t.Namespace, Name: t.Name, UID: t.UID,
			PodIP: t.PodIP, PodIPs: append([]string(nil), t.PodIPs...),
		})
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
