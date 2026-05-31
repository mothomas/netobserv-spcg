package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// ResolveSessionID reads bearer (legacy) or SPCG session header.
func ResolveSessionID(r *http.Request) (string, Mode, string, error) {
	if sid := strings.TrimSpace(r.Header.Get(HeaderSPCGSession)); sid != "" {
		return sid, "", "", nil
	}
	tok, err := ExtractUserToken(r)
	if err != nil {
		return "", "", "", err
	}
	return "", ModeBearer, tok, nil
}

// DecodeKubeconfigUpload accepts raw YAML or standard base64 payload from the UI.
func DecodeKubeconfigUpload(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("kubeconfig payload is empty")
	}
	if !strings.Contains(raw, "apiVersion:") {
		dec, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig must be YAML or valid base64: %w", err)
		}
		raw = string(dec)
	}
	return []byte(raw), nil
}
