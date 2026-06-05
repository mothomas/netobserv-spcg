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
	oauthRouteNS   = "openshift-authentication"
	oauthRouteName = "oauth-openshift"
	// Legacy in-cluster name; not present on all OCP builds (DNS NXDOMAIN). Prefer oauth-openshift Route host.
	legacyInternalOAuthTokenURL = "https://oauth.openshift.svc.cluster.local/oauth/token"
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
	landingNS := envOrDefault("SPCG_LANDING_NAMESPACE", spcgNS)
	controlNS := envOrDefault("SPCG_CONTROL_NAMESPACE", spcgNS)
	oauthHost, err := routeHTTPSHost(ctx, cfg, oauthRouteNS, oauthRouteName)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("oauth route: %w", err)
	}
	uiHost, err := routeHTTPSHost(ctx, cfg, landingNS, "spcg")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("spcg route: %w", err)
	}
	// Prefer dedicated API route when present; otherwise single-host (Argo CD–style) UI route + Next /api proxy.
	publicAPIBase = uiHost
	if apiHost, err := routeHTTPSHost(ctx, cfg, controlNS, "spcg-api"); err == nil {
		publicAPIBase = apiHost
	}
	authorizeURL = oauthHost + "/oauth/authorize"
	// Same host as authorize (matches openshift.default.svc/.well-known/oauth-authorization-server).
	tokenURL = oauthHost + "/oauth/token"
	if override := strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL")); override != "" {
		tokenURL = override
	} else if useLegacy := strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL_LEGACY_INTERNAL")); useLegacy == "true" || useLegacy == "1" {
		tokenURL = legacyInternalOAuthTokenURL
	}
	// Monolithic overlay: callback on UI host (Next.js proxies /api). Secure split: callback on API Route host.
	redirectURL = uiHost + "/api/v1/auth/openshift/callback"
	if publicAPIBase != "" && publicAPIBase != uiHost {
		redirectURL = publicAPIBase + "/api/v1/auth/openshift/callback"
	}
	frontendURL = uiHost
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

	needDiscover := cfg.AuthorizeURL == "" || cfg.TokenURL == "" || cfg.RedirectURL == "" || cfg.FrontendURL == "" || publicAPI == ""
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
	if publicAPI == "" && cfg.FrontendURL != "" {
		publicAPI = cfg.FrontendURL
	}
	cfg.PublicAPIBase = publicAPI
	return cfg, true, nil
}

// IngressBaseURL builds the public https://host from a request (X-Forwarded-* when proxied via spcg-frontend).
func IngressBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.Index(host, ","); idx > 0 {
		host = strings.TrimSpace(host[:idx])
	}
	host = strings.TrimSuffix(host, "/")
	if host == "" || strings.Contains(host, ".svc") || strings.Contains(host, ".cluster.local") {
		return ""
	}
	proto := "https"
	if r.TLS == nil {
		if p := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); p != "" {
			proto = p
		} else {
			proto = "http"
		}
	}
	return proto + "://" + host
}

// CallbackRedirectURLFromRequest is the OAuth redirect_uri matching the browser-visible Route host.
func CallbackRedirectURLFromRequest(r *http.Request) string {
	base := IngressBaseURL(r)
	if base == "" {
		return ""
	}
	return strings.TrimSuffix(base, "/") + "/api/v1/auth/openshift/callback"
}
