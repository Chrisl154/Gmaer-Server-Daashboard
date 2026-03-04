package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service exposes Prometheus metrics
type Service struct {
	registry      *prometheus.Registry
	serversTotal  prometheus.Gauge
	serversUp     prometheus.Gauge
	deployTotal   *prometheus.CounterVec
	backupTotal   *prometheus.CounterVec
	apiRequests   *prometheus.CounterVec
	apiLatency    *prometheus.HistogramVec
}

// NewService creates a new metrics service
func NewService() *Service {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	svc := &Service{
		registry: reg,
		serversTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "games_servers_total",
			Help: "Total number of managed game servers",
		}),
		serversUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "games_servers_running",
			Help: "Number of currently running game servers",
		}),
		deployTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "games_deploys_total",
			Help: "Total deployments by adapter and method",
		}, []string{"adapter", "method", "status"}),
		backupTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "games_backups_total",
			Help: "Total backups by server and type",
		}, []string{"server_id", "type", "status"}),
		apiRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "games_api_requests_total",
			Help: "Total API requests",
		}, []string{"method", "path", "status"}),
		apiLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "games_api_request_duration_seconds",
			Help:    "API request latency",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}

	reg.MustRegister(
		svc.serversTotal,
		svc.serversUp,
		svc.deployTotal,
		svc.backupTotal,
		svc.apiRequests,
		svc.apiLatency,
	)

	return svc
}

// Handler returns the Prometheus metrics HTTP handler
func (s *Service) Handler() http.Handler {
	return promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})
}

// RecordDeploy increments the deploy counter
func (s *Service) RecordDeploy(adapter, method, status string) {
	s.deployTotal.WithLabelValues(adapter, method, status).Inc()
}

// RecordBackup increments the backup counter
func (s *Service) RecordBackup(serverID, backupType, status string) {
	s.backupTotal.WithLabelValues(serverID, backupType, status).Inc()
}

// SetServersTotal updates the total servers gauge
func (s *Service) SetServersTotal(n float64) {
	s.serversTotal.Set(n)
}

// SetServersRunning updates the running servers gauge
func (s *Service) SetServersRunning(n float64) {
	s.serversUp.Set(n)
}
