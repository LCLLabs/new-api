package obsmetrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func InstallRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()
	registry := prometheus.NewRegistry()
	inst := newInstruments(registry, registry)
	mu.Lock()
	previous := active
	active = inst
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		active = previous
		mu.Unlock()
	})
	return registry
}
