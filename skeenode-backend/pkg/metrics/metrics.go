package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for Skeenode.
// Using promauto for automatic registration with default registry.
var (
	// --- Job Metrics ---

	// JobsTotal counts total jobs by status.
	JobsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "skeenode",
			Subsystem: "jobs",
			Name:      "total",
			Help:      "Total number of jobs by status",
		},
		[]string{"status"},
	)

	// ExecutionsTotal counts total executions by status.
	ExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "executions",
			Name:      "total",
			Help:      "Total number of job executions by status",
		},
		[]string{"status", "job_type"},
	)

	// ExecutionDuration tracks job execution duration.
	ExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "skeenode",
			Subsystem: "executions",
			Name:      "duration_seconds",
			Help:      "Duration of job executions in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 15), // 0.1s to ~1.8h
		},
		[]string{"job_name", "status"},
	)

	// --- Scheduler Metrics ---

	// SchedulerLag measures delay between scheduled time and actual dispatch.
	SchedulerLag = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "skeenode",
			Subsystem: "scheduler",
			Name:      "lag_seconds",
			Help:      "Delay between scheduled time and actual dispatch",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		},
	)

	// SchedulerPolls counts scheduler poll cycles.
	SchedulerPolls = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "scheduler",
			Name:      "polls_total",
			Help:      "Total number of scheduler poll cycles",
		},
	)

	// JobsDispatched counts jobs dispatched per cycle.
	JobsDispatched = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "scheduler",
			Name:      "jobs_dispatched_total",
			Help:      "Total number of jobs dispatched",
		},
	)

	// --- Executor Metrics ---

	// ActiveNodes tracks number of active executor nodes.
	ActiveNodes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "skeenode",
			Subsystem: "cluster",
			Name:      "active_nodes",
			Help:      "Number of active executor nodes",
		},
	)

	// ExecutorJobsRunning tracks concurrent jobs on executor.
	ExecutorJobsRunning = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "skeenode",
			Subsystem: "executor",
			Name:      "jobs_running",
			Help:      "Number of currently running jobs on this executor",
		},
	)

	// HeartbeatsSent counts heartbeats sent by executor.
	HeartbeatsSent = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "executor",
			Name:      "heartbeats_total",
			Help:      "Total heartbeats sent",
		},
	)

	// --- Queue Metrics ---

	// QueueDepth tracks pending jobs in queue.
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "skeenode",
			Subsystem: "queue",
			Name:      "pending_jobs",
			Help:      "Number of jobs pending in the queue",
		},
	)

	// --- Retry Metrics ---

	// RetriesTotal counts job retries.
	RetriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "executions",
			Name:      "retries_total",
			Help:      "Total number of job retries",
		},
		[]string{"job_name"},
	)

	// OrphansReaped counts orphaned executions cleaned up.
	OrphansReaped = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "scheduler",
			Name:      "orphans_reaped_total",
			Help:      "Total number of orphaned executions cleaned up",
		},
	)
)

// RecordExecution records metrics for a completed execution.
func RecordExecution(jobName, jobType, status string, durationSeconds float64) {
	ExecutionsTotal.WithLabelValues(status, jobType).Inc()
	ExecutionDuration.WithLabelValues(jobName, status).Observe(durationSeconds)
}

// RecordDispatch records a job being dispatched.
func RecordDispatch(lagSeconds float64) {
	JobsDispatched.Inc()
	SchedulerLag.Observe(lagSeconds)
}
