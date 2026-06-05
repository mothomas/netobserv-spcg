package k8s

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// RESTConfigFromBearerToken returns cluster REST config using the user token.
func RESTConfigFromBearerToken(userToken string) (*rest.Config, error) {
	if userToken == "" {
		return nil, fmt.Errorf("bearer token is required")
	}
	return restConfigForUserBearerToken(userToken)
}

// RESTConfigFromKubeconfigYAML parses kubeconfig bytes into REST config.
func RESTConfigFromKubeconfigYAML(kubeconfigYAML []byte) (*rest.Config, error) {
	if len(kubeconfigYAML) == 0 {
		return nil, fmt.Errorf("kubeconfig content is empty")
	}
	return clientcmd.RESTConfigFromKubeConfig(kubeconfigYAML)
}
