package k8s

import "k8s.io/client-go/kubernetes"

// ClientsetWrap aliases kubernetes.Interface for portal handlers.
type ClientsetWrap struct {
	kubernetes.Interface
}
