package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var Registry = prometheus.NewRegistry()

var (
	JobsEnqueued = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "taskqueue_jobs_enqueued_total",
		Help: "Total number of jobs enqueued",
	})

	JobsCompleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "taskqueue_jobs_completed_total",
		Help: "Total number of jobs completed successfully",
	})

	JobsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "taskqueue_jobs_failed_total",
		Help: "Total number of jobs permanently failed",
	})

	JobProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "taskqueue_job_processing_duration_seconds",
		Help:    "Time taken to process jobs",
		Buckets: prometheus.DefBuckets,
	})

	JobProcessingTimeLatest = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "taskqueue_job_processing_duration_seconds_latest",
		Help: "Duration of the most recently processed job",
	})

	JobProcessingTimeAverage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "taskqueue_job_processing_duration_seconds_average",
		Help: "Average processing time of all completed jobs",
	})

	QueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "taskqueue_queue_depth",
		Help: "Number of jobs currently waiting in the queue",
	})

	DLQSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "taskqueue_dlq_size",
		Help: "Number of jobs in the dead letter queue",
	})
)

func Register() {
	Registry.MustRegister(JobsEnqueued, JobsCompleted, JobsFailed, JobProcessingTime, JobProcessingTimeLatest, JobProcessingTimeAverage, QueueDepth, DLQSize)
}
