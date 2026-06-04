package auth

import (
	"net/http"
	"testing"
)

func TestOAuthRoundTripperInsecureSkipVerify(t *testing.T) {
	t.Setenv("OAUTH_TLS_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("OAUTH_CA_BUNDLE", "")

	tr, ok := oauthRoundTripper().(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=true when OAUTH_TLS_INSECURE_SKIP_VERIFY=true")
	}
}

func TestOAuthRoundTripperDefaultWithoutEnv(t *testing.T) {
	t.Setenv("OAUTH_TLS_INSECURE_SKIP_VERIFY", "")
	t.Setenv("OAUTH_CA_BUNDLE", "")

	tr, ok := oauthRoundTripper().(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("unexpected InsecureSkipVerify without oauth TLS env")
	}
}
