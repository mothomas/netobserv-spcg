package auth

import (
	"os"
	"testing"
)

func TestMethodAllowed(t *testing.T) {
	t.Setenv("SPCG_AUTH_METHODS", "openshift,kubeconfig")
	if !MethodAllowed("openshift") {
		t.Fatal("expected openshift")
	}
	if !MethodAllowed("kubeconfig") {
		t.Fatal("expected kubeconfig")
	}
	if MethodAllowed("token") {
		t.Fatal("token should be disabled")
	}
}

func TestAllowedMethodsDefault(t *testing.T) {
	os.Unsetenv("SPCG_AUTH_METHODS")
	m := AllowedMethods()
	if len(m) != 2 {
		t.Fatalf("default methods: %v", m)
	}
}
