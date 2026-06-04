package k8s

import (
	"fmt"
	"net/http"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ImpersonatedClientset builds a client-go REST config that acts as the end user.
func ImpersonatedClientset(userToken string) (*kubernetes.Clientset, *rest.Config, error) {
	if userToken == "" {
		return nil, nil, fmt.Errorf("user token is required for impersonated API access")
	}

	cfg, err := restConfigForUserBearerToken(userToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed building cluster REST config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating impersonated kubernetes client: %w", err)
	}
	return cs, cfg, nil
}

type impersonationRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t *impersonationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

// restConfigForUserBearerToken builds in-cluster API config that sends the user OAuth/kube token,
// not the pod service account (InClusterConfig sets BearerTokenFile which overrides BearerToken).
func restConfigForUserBearerToken(userToken string) (*rest.Config, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	cfg.BearerToken = userToken
	cfg.BearerTokenFile = ""
	cfg.ExecProvider = nil
	cfg.AuthProvider = nil
	cfg.Impersonate = rest.ImpersonationConfig{}
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return &impersonationRoundTripper{base: rt, token: userToken}
	}
	return cfg, nil
}

func restConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// PrivilegedInCluster returns a clientset using the pod service account (engine tier).
func PrivilegedInCluster() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed in-cluster config for engine: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed privileged kubernetes client: %w", err)
	}
	return cs, nil
}
