package auth

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	HeaderForwardedUserToken = "X-Forwarded-User-Token"
)

// ExtractUserToken reads OpenShift OAuth bearer from standard headers.
func ExtractUserToken(r *http.Request) (string, error) {
	if xf := strings.TrimSpace(r.Header.Get(HeaderForwardedUserToken)); xf != "" {
		return xf, nil
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return "", fmt.Errorf("missing bearer token: set Authorization or %s", HeaderForwardedUserToken)
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", fmt.Errorf("authorization header must be Bearer token")
	}
	tok := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if tok == "" {
		return "", fmt.Errorf("empty bearer token")
	}
	return tok, nil
}

// Wipe overwrites a byte slice in memory (best-effort zero retention).
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// WipeString overwrites string backing via copy to []byte when possible.
func WipeString(s *string) {
	if s == nil {
		return
	}
	b := []byte(*s)
	Wipe(b)
	*s = ""
}
