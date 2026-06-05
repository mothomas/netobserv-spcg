package auth

import "testing"

func TestOAuthStateRedirectBinding(t *testing.T) {
	s := newOAuthStateStore()
	state, err := s.IssueRedirect("https://spcg.example.com/api/v1/auth/openshift/callback")
	if err != nil {
		t.Fatal(err)
	}
	uri, ok := s.ConsumeRedirect(state)
	if !ok || uri != "https://spcg.example.com/api/v1/auth/openshift/callback" {
		t.Fatalf("got ok=%v uri=%q", ok, uri)
	}
	if _, ok := s.ConsumeRedirect(state); ok {
		t.Fatal("state should be single-use")
	}
}
