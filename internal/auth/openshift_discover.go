package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

const (
	oauthRouteNS  = "openshift-authentication"
	oauthRouteName = "oauth-openshift"
	internalOAuthTokenURL = "https://oauth.openshift.svc.cluster.local/oauth/token"
)

// DiscoverOpenShiftRoutes fills OAuth/UI URLs from OpenShift Route objects (Argo CD–style; no user-entered domain).
func DiscoverOpenShiftRoutes(ctx context.Context, spcgNS string) (authorizeURL, tokenURL, redirectURL, frontendURL, publicAPIBase string, err error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", "", "", "", "", err
	}
	if spcgNS == "" {
		spcgNS = envOrDefault("SPCG_NAMESPACE", "pcap-frontend")
	}
	oauthHost, err := routeHTTPSHost(ctx, cfg, oauthRouteNS, oauthRouteName)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("oauth route: %w", err)
	}
	apiHost, err := routeHTTPSHost(ctx, cfg, spcgNS, "spcg-api")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("spcg-api route: %w", err)
	}
	uiHost, err := routeHTTPSHost(ctx, cfg, spcgNS, "spcg")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("spcg route: %w", err)
	}
	authorizeURL = oauthHost + "/oauth/authorize"
	tokenURL = internalOAuthTokenURL
	if override := strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL")); override != "" {
		tokenURL = override
	}
	redirectURL = apiHost + "/api/v1/auth/openshift/callback"
	frontendURL = uiHost
	publicAPIBase = apiHost
	return authorizeURL, tokenURL, redirectURL, frontendURL, publicAPIBase, nil
}

func envOrDefault(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func routeHTTPSHost(ctx context.Context, cfg *rest.Config, ns, name string) (string, error) {
	cfg = rest.CopyConfig(cfg)
	cfg.APIPath = "/apis"
	gv := schema.GroupVersion{Group: "route.openshift.io", Version: "v1"}
	cfg.GroupVersion = &gv
	tr, err := rest.TransportFor(cfg)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/apis/route.openshift.io/v1/namespaces/%s/routes/%s", ns, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Host+path, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	var obj struct {
		Spec struct {
			Host string `json:"host"`
		} `json:"spec"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return "", err
	}
	host := strings.TrimSpace(obj.Spec.Host)
	if host == "" {
		return "", fmt.Errorf("route %s/%s has empty spec.host", ns, name)
	}
	if strings.Contains(host, "://") {
		return host, nil
	}
	return "https://" + host, nil
}

// ResolveOAuthSettings merges env vars with in-cluster Route discovery (like Argo CD on OpenShift).
func ResolveOAuthSettings(ctx context.Context) (OAuthSettings, bool, error) {
	if !MethodAllowed(ModeOpenShift) {
		return OAuthSettings{}, false, nil
	}
	clientID := strings.TrimSpace(os.Getenv("OAUTH_CLIENT_ID"))
	if clientID == "" {
		clientID = "spcg-ui"
	}
	scope := strings.TrimSpace(os.Getenv("OAUTH_SCOPE"))
	if scope == "" {
		scope = "user:full"
	}
	cfg := OAuthSettings{
		AuthorizeURL: strings.TrimSpace(os.Getenv("OAUTH_AUTHORIZE_URL")),
		TokenURL:     strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL")),
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(os.Getenv("OAUTH_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_URL")),
		Scope:        scope,
		FrontendURL:  strings.TrimSpace(os.Getenv("SPCG_FRONTEND_URL")),
	}
	publicAPI := strings.TrimSuffix(strings.TrimSpace(os.Getenv("SPCG_PUBLIC_API_BASE")), "/")

	needDiscover := cfg.AuthorizeURL == "" || cfg.RedirectURL == "" || cfg.FrontendURL == "" || publicAPI == ""
	if needDiscover {
		authz, tok, redir, front, apiBase, derr := DiscoverOpenShiftRoutes(ctx, "")
		if derr == nil {
			if cfg.AuthorizeURL == "" {
				cfg.AuthorizeURL = authz
			}
			if cfg.TokenURL == "" {
				cfg.TokenURL = tok
			}
			if cfg.RedirectURL == "" {
				cfg.RedirectURL = redir
			}
			if cfg.FrontendURL == "" {
				cfg.FrontendURL = front
			}
			if publicAPI == "" {
				publicAPI = apiBase
			}
		} else if cfg.AuthorizeURL == "" || cfg.RedirectURL == "" || cfg.FrontendURL == "" {
			return OAuthSettings{}, true, derr
		}
	}
	if err := cfg.Valid(); err != nil {
		return OAuthSettings{}, true, err
	}
	cfg.PublicAPIBase = publicAPI
	return cfg, true, nil
}
