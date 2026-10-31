// Package metrics owns the Prometheus registry and the metrics listener
// (HORIZON_METRICS_ADDR, docs/operations.md#health).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Namespace prefixes every Horizon metric.
const Namespace = "horizon"

// Registry is the process-wide registry, with Go and process collectors.
type Registry struct {
	*prometheus.Registry
}

// New returns a registry with the standard collectors registered.
func New() *Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return &Registry{Registry: r}
}

// Handler serves /metrics for this registry.
func (r *Registry) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{Registry: r.Registry}))
	return mux
}
