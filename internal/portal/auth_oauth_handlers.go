package portal

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authenticationv1 "k8s.io/api/authentication/v1"

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
	if !auth.MethodAllowed(string(auth.ModeOpenShift)) {
		writeJSON(w, out)
		return
	}
	osCfg := map[string]string{
		"authorize_path": "/api/v1/auth/openshift/authorize",
	}
	cfg, ok, err := auth.ResolveOAuthSettings(r.Context())
	if err != nil {
		osCfg["error"] = err.Error()
		if base := auth.IngressBaseURL(r); base != "" {
			osCfg["authorize_url"] = base + "/api/v1/auth/openshift/authorize"
			osCfg["redirect_hint"] = base + "/api/v1/auth/openshift/callback"
		}
		out["openshift"] = osCfg
		writeJSON(w, out)
		return
	}
	if ok {
		uiBase := strings.TrimSuffix(cfg.FrontendURL, "/")
		apiBase := strings.TrimSuffix(cfg.PublicAPIBase, "/")
		if cfg.RedirectURL != "" {
			osCfg["redirect_uri"] = cfg.RedirectURL
		}
		if apiBase != "" && apiBase != uiBase {
			out["public_api_base"] = apiBase
			osCfg["authorize_url"] = apiBase + "/api/v1/auth/openshift/authorize"
		} else if uiBase != "" {
			osCfg["authorize_url"] = uiBase + "/api/v1/auth/openshift/authorize"
		} else if base := auth.IngressBaseURL(r); base != "" {
			osCfg["authorize_url"] = base + "/api/v1/auth/openshift/authorize"
		}
		out["openshift"] = osCfg
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
	cfg, ok, err := auth.ResolveOAuthSettings(r.Context())
	if !ok {
		http.Error(w, "OpenShift OAuth is not enabled", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
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
	cfg, ok, err := auth.ResolveOAuthSettings(r.Context())
	if !ok {
		http.Error(w, "OpenShift OAuth is not enabled", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
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
	accessToken, err := auth.ExchangeCodeForToken(auth.OAuthHTTPClient(), cfg, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	cs, err := spcgk8s.ClientsetFromBearerToken(accessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := cs.AuthenticationV1().SelfSubjectReviews().Create(r.Context(), &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{}); err != nil {
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
