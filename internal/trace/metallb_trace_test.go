package trace

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAdMatchesPoolExplicit(t *testing.T) {
	ad := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"ipAddressPools": []interface{}{"pool-a", "pool-b"},
		},
	}}
	if !adMatchesPool(ad, "pool-a") {
		t.Fatal("expected pool-a match")
	}
	if adMatchesPool(ad, "pool-z") {
		t.Fatal("expected pool-z miss")
	}
}

func TestAdMatchesPoolDefaultAll(t *testing.T) {
	ad := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{},
	}}
	if !adMatchesPool(ad, "any-pool") {
		t.Fatal("empty pool list should match all pools")
	}
}
