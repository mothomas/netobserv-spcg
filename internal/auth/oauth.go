package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
	ttl    time.Duration
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{
		states: make(map[string]time.Time),
		ttl:    10 * time.Minute,
	}
}

// IssueOAuthState creates a CSRF state for the authorization redirect.
func (s *oauthStateStore) Issue() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, t := range s.states {
		if now.Sub(t) > s.ttl {
			delete(s.states, k)
		}
	}
	s.states[state] = now
	return state, nil
}

// ConsumeOAuthState validates and burns a CSRF state from the callback.
func (s *oauthStateStore) Consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Since(t) <= s.ttl
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
	q.Set("scope", cfg.Scope)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCodeForToken performs the authorization_code grant against the OAuth token endpoint.
func ExchangeCodeForToken(ctx http.Client, cfg OAuthSettings, code string) (string, error) {
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
	resp, err := ctx.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint %s: %s", resp.Status, strings.TrimSpace(string(body)))
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
