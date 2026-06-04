package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientsetFromBearerToken uses the end-user OAuth/service account token directly.
func ClientsetFromBearerToken(userToken string) (*kubernetes.Clientset, error) {
	if userToken == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	cfg, err := restConfigForUserBearerToken(userToken)
	if err != nil {
		return nil, fmt.Errorf("failed building cluster REST config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed creating kubernetes client from bearer token: %w", err)
	}
	return cs, nil
}

// ClientsetFromKubeconfig uses credentials embedded in the user's kubeconfig (kubectl identity).
func ClientsetFromKubeconfig(kubeconfigYAML []byte) (*kubernetes.Clientset, *rest.Config, error) {
	if len(kubeconfigYAML) == 0 {
		return nil, nil, fmt.Errorf("kubeconfig content is empty")
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigYAML)
	if err != nil {
		return nil, nil, fmt.Errorf("failed parsing kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating kubernetes client from kubeconfig: %w", err)
	}
	return cs, cfg, nil
}

// ClusterHost returns the API server host from config (for UI display).
func ClusterHost(cfg *rest.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.Host
}
