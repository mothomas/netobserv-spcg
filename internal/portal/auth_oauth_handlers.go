package portal

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/netobserv/spcg/internal/auth"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

func (s *Server) handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	methods := auth.AllowedMethods()
	out := map[string]interface{}{
		"methods": methods,
	}
	if auth.MethodAllowed(string(auth.ModeOpenShift)) {
		if _, ok := auth.LoadOAuthSettings(); ok {
			out["openshift"] = map[string]string{
				"authorize_path": "/api/v1/auth/openshift/authorize",
			}
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleOpenShiftAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !auth.MethodAllowed(string(auth.ModeOpenShift)) {
		http.Error(w, "openshift login is disabled", http.StatusNotFound)
		return
	}
	cfg, ok := auth.LoadOAuthSettings()
	if !ok {
		http.Error(w, "OpenShift OAuth is not configured (set OAUTH_CLIENT_ID and related env)", http.StatusServiceUnavailable)
		return
	}
	state, err := auth.OAuthState.Issue()
	if err != nil {
		http.Error(w, "failed to start oauth", http.StatusInternalServerError)
		return
	}
	redirectURL, err := auth.AuthorizeRedirectURL(cfg, state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) handleOpenShiftCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !auth.MethodAllowed(string(auth.ModeOpenShift)) {
		http.Error(w, "openshift login is disabled", http.StatusNotFound)
		return
	}
	cfg, ok := auth.LoadOAuthSettings()
	if !ok {
		http.Error(w, "OpenShift OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("oauth error: %s %s", errParam, desc), http.StatusUnauthorized)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	if !auth.OAuthState.Consume(state) {
		http.Error(w, "invalid or expired oauth state", http.StatusBadRequest)
		return
	}
	client := http.Client{Timeout: 30 * time.Second}
	accessToken, err := auth.ExchangeCodeForToken(client, cfg, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	cs, err := spcgk8s.ClientsetFromBearerToken(accessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := cs.CoreV1().Namespaces().List(r.Context(), metav1.ListOptions{Limit: 1}); err != nil {
		http.Error(w, fmt.Sprintf("token rejected by API server: %v", err), http.StatusUnauthorized)
		return
	}
	if s.Sessions == nil {
		s.Sessions = auth.NewStore(0)
	}
	sess, err := s.Sessions.CreateBearer(accessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clusterLabel := strings.TrimSpace(os.Getenv("SPCG_CLUSTER_DISPLAY_NAME"))
	if clusterLabel == "" {
		clusterLabel = "OpenShift"
	}
	redirect := auth.FrontendCallbackURL(cfg, sess.ID, clusterLabel)
	http.Redirect(w, r, redirect, http.StatusFound)
}
