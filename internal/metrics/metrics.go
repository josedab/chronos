// Package metrics provides Prometheus metrics for Chronos.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for Chronos.
type Metrics struct {
	// Job metrics
	JobsTotal   prometheus.Gauge
	JobsEnabled prometheus.Gauge

	// Execution metrics
	ExecutionsTotal   *prometheus.CounterVec
	ExecutionDuration *prometheus.HistogramVec

	// Scheduler metrics
	SchedulerTickDuration prometheus.Histogram
	ScheduledRunsTotal    prometheus.Counter
	MissedRunsTotal       prometheus.Counter

	// Raft metrics
	RaftIsLeader prometheus.Gauge
	RaftPeers    prometheus.Gauge

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
}

// New creates a new Metrics instance and registers with Prometheus.
func New() *Metrics {
	m := &Metrics{
		JobsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chronos",
			Name:      "jobs_total",
			Help:      "Total number of jobs.",
		}),
		JobsEnabled: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chronos",
			Name:      "jobs_enabled",
			Help:      "Number of enabled jobs.",
		}),
		ExecutionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "chronos",
			Name:      "executions_total",
			Help:      "Total number of job executions.",
		}, []string{"job_id", "job_name", "status"}),
		ExecutionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "chronos",
			Name:      "execution_duration_seconds",
			Help:      "Job execution duration in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 15), // 0.1s to ~1h
		}, []string{"job_id", "job_name"}),
		SchedulerTickDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "chronos",
			Name:      "scheduler_tick_duration_seconds",
			Help:      "Scheduler tick duration in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
		}),
		ScheduledRunsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chronos",
			Name:      "scheduled_runs_total",
			Help:      "Total number of scheduled runs.",
		}),
		MissedRunsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chronos",
			Name:      "missed_runs_total",
			Help:      "Total number of missed runs.",
		}),
		RaftIsLeader: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chronos",
			Name:      "raft_is_leader",
			Help:      "Whether this node is the Raft leader (1 = leader, 0 = follower).",
		}),
		RaftPeers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chronos",
			Name:      "raft_peers",
			Help:      "Number of Raft peers in the cluster.",
		}),
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "chronos",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "chronos",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.JobsTotal,
		m.JobsEnabled,
		m.ExecutionsTotal,
		m.ExecutionDuration,
		m.SchedulerTickDuration,
		m.ScheduledRunsTotal,
		m.MissedRunsTotal,
		m.RaftIsLeader,
		m.RaftPeers,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
	)

	return m
}

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordExecution records an execution metric.
func (m *Metrics) RecordExecution(jobID, jobName, status string, duration float64) {
	m.ExecutionsTotal.WithLabelValues(jobID, jobName, status).Inc()
	m.ExecutionDuration.WithLabelValues(jobID, jobName).Observe(duration)
}

// RecordHTTPRequest records an HTTP request metric.
func (m *Metrics) RecordHTTPRequest(method, path, status string, duration float64) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// SetLeader sets the leader status metric.
func (m *Metrics) SetLeader(isLeader bool) {
	if isLeader {
		m.RaftIsLeader.Set(1)
	} else {
		m.RaftIsLeader.Set(0)
	}
}

// SetJobCounts sets the job count metrics.
func (m *Metrics) SetJobCounts(total, enabled int) {
	m.JobsTotal.Set(float64(total))
	m.JobsEnabled.Set(float64(enabled))
}

// SetPeerCount sets the peer count metric.
func (m *Metrics) SetPeerCount(count int) {
	m.RaftPeers.Set(float64(count))
}
