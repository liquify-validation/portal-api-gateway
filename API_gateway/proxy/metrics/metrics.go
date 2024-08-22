package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	MetricRequestsAPI = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_by_api_key",
			Help: "Number of HTTP requests by API key, organization, organization ID, chain, and status.",
		}, []string{"api_key", "org", "org_id", "chain", "status"},
	)

	MetricAPICache = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits",
			Help: "Number of calls with cached API key.",
		}, []string{"state"},
	)

	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"status_code"},
	)
)

func InitPrometheusMetrics() {
	prometheus.MustRegister(MetricRequestsAPI)
	prometheus.MustRegister(MetricAPICache)
	prometheus.MustRegister(RequestsTotal)
}
