package auth

import (
	"os"
	"strings"
)

// AllowedMethods returns login modes enabled for this deployment (env SPCG_AUTH_METHODS).
// Examples: "openshift", "kubeconfig", "openshift,kubeconfig". Empty env defaults to kubeconfig+token for dev.
func AllowedMethods() []string {
	raw := strings.TrimSpace(os.Getenv("SPCG_AUTH_METHODS"))
	if raw == "" {
		return []string{string(ModeKubeconfig), string(ModeBearer)}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func MethodAllowed(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "token" {
		mode = string(ModeBearer)
	}
	for _, m := range AllowedMethods() {
		if m == mode || (mode == string(ModeBearer) && m == "token") {
			return true
		}
	}
	return false
}
