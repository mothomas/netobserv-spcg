package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const ModeOpenShift = "openshift"

// OAuthSettings from environment (OpenShift OAuth client).
type OAuthSettings struct {
	AuthorizeURL  string
	TokenURL      string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scope         string
	FrontendURL   string
	PublicAPIBase string
}

// LoadOAuthSettings returns static env-only settings (no discovery). Prefer ResolveOAuthSettings in production.
func LoadOAuthSettings() (OAuthSettings, bool) {
	cfg, ok, err := ResolveOAuthSettings(context.Background())
	if err != nil || !ok {
		return OAuthSettings{}, false
	}
	return cfg, true
}

func (o OAuthSettings) Valid() error {
	if o.AuthorizeURL == "" || o.TokenURL == "" {
		return fmt.Errorf("OAUTH_AUTHORIZE_URL and OAUTH_TOKEN_URL are required")
	}
	if o.RedirectURL == "" {
		return fmt.Errorf("OAUTH_REDIRECT_URL is required")
	}
	if o.FrontendURL == "" {
		return fmt.Errorf("SPCG_FRONTEND_URL is required for OAuth callback redirect")
	}
	return nil
}

type oauthPending struct {
	issuedAt    time.Time
	redirectURI string
}

type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]oauthPending
	ttl    time.Duration
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{
		states: make(map[string]oauthPending),
		ttl:    10 * time.Minute,
	}
}

func (s *oauthStateStore) pruneLocked(now time.Time) {
	for k, p := range s.states {
		if now.Sub(p.issuedAt) > s.ttl {
			delete(s.states, k)
		}
	}
}

// IssueRedirect creates CSRF state bound to redirect_uri used at authorize (must match token exchange).
func (s *oauthStateStore) IssueRedirect(redirectURI string) (string, error) {
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		return "", fmt.Errorf("redirect_uri required for oauth state")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.pruneLocked(now)
	s.states[state] = oauthPending{issuedAt: now, redirectURI: redirectURI}
	return state, nil
}

// ConsumeRedirect validates state and returns the redirect_uri from authorize.
func (s *oauthStateStore) ConsumeRedirect(state string) (redirectURI string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, found := s.states[state]
	if !found {
		return "", false
	}
	delete(s.states, state)
	if time.Since(p.issuedAt) > s.ttl {
		return "", false
	}
	return p.redirectURI, true
}

// OAuthState is a process-wide CSRF store for the authorization code flow.
var OAuthState = newOAuthStateStore()

// AuthorizeRedirectURL builds the OpenShift OAuth authorize URL.
func AuthorizeRedirectURL(cfg OAuthSettings, state string) (string, error) {
	if err := cfg.Valid(); err != nil {
		return "", err
	}
	u, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", cfg.RedirectURL)
	// Omit scope unless set — OpenShift returns invalid_request when scope violates OAuthClient scopeRestrictions.
	if strings.TrimSpace(cfg.Scope) != "" {
		q.Set("scope", cfg.Scope)
	}
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func oauthTLSInsecureSkipVerify() bool {
	v := strings.TrimSpace(os.Getenv("OAUTH_TLS_INSECURE_SKIP_VERIFY"))
	return v == "true" || v == "1" || strings.EqualFold(v, "yes")
}

func oauthTLSConfig() *tls.Config {
	insecure := oauthTLSInsecureSkipVerify()
	caPath := strings.TrimSpace(os.Getenv("OAUTH_CA_BUNDLE"))
	if !insecure && caPath == "" {
		return nil
	}
	cfg := &tls.Config{}
	if insecure {
		cfg.InsecureSkipVerify = true
	}
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return cfg
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		pool.AppendCertsFromPEM(pem)
		cfg.RootCAs = pool
	}
	return cfg
}

func oauthRoundTripper() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	} else {
		base = base.Clone()
	}
	if tlsCfg := oauthTLSConfig(); tlsCfg != nil {
		base.TLSClientConfig = tlsCfg
	}
	return base
}

// OAuthHTTPClient is used for token exchange (Argo CD Dex uses insecureCA similarly on OpenShift).
func OAuthHTTPClient() http.Client {
	return http.Client{Timeout: 30 * time.Second, Transport: oauthRoundTripper()}
}

// ExchangeCodeForToken performs the authorization_code grant against the OAuth token endpoint.
func ExchangeCodeForToken(client http.Client, cfg OAuthSettings, code string) (string, error) {
	if err := cfg.Valid(); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequest(http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if strings.Contains(msg, "unauthorized_client") {
			return "", fmt.Errorf("token endpoint %s: %s — sync OAuthClient %q secret with secret %s (run openshift-secure-fix-oauth.sh)",
				resp.Status, msg, cfg.ClientID, "spcg-oauth-client")
		}
		return "", fmt.Errorf("token endpoint %s: %s", resp.Status, msg)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	return tok.AccessToken, nil
}

// FrontendCallbackURL returns where to send the browser after successful OAuth.
func FrontendCallbackURL(cfg OAuthSettings, sessionID, cluster string) string {
	u, _ := url.Parse(cfg.FrontendURL)
	u.Path = strings.TrimSuffix(u.Path, "/") + "/auth/callback"
	q := u.Query()
	q.Set("session_id", sessionID)
	if cluster != "" {
		q.Set("cluster", cluster)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
