package controller

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder knows how to record queue metrics.
type Recorder interface {
	// AddEvent adds an event metric records of a queued event.
	AddEvent(ctx context.Context, name string, retry bool)
	// GetEvent measures how long an object stays in the queue. If the object
	// is already in queue it will be measured once, since the first time it
	// was added to the queue.
	GetEvent(ctx context.Context, name string, queued time.Time)
	// DoneEvent measures how long it takes to process a resources.
	DoneEvent(ctx context.Context, name string, success bool, start time.Time)
	// RegisterLen will register a function that will be called by the metrics
	// recorder to get the length of a queue at a given point in time.
	RegisterLen(name string, call func(context.Context) int)
}

// RecorderConfig is the recorder configuration.
type RecorderConfig struct {
	// Namespace is the prometheus metrics namespace.
	Namespace string
	// Subsystem is the prometheus metrics subsystem.
	Subsystem string
	// QueueBuckets sets custom buckets for the duration/latency items in queue metrics.
	// Check https://godoc.org/github.com/prometheus/client_golang/prometheus#pkg-variables
	QueueBuckets []float64 `default:"[0.01,0.05,0.1,0.25,0.5,1,3,10,20,60,150,300]"`
	// ProcessBuckets sets custom buckets for the duration/latency processing metrics.
	// Check https://godoc.org/github.com/prometheus/client_golang/prometheus#pkg-variables
	ProcessBuckets []float64 `default:"[0.005,0.01,0.025,0.05,0.1,0.25,0.5,1,2.5,5,10,30]"`
}

// defaults will set default values for the recorder config.
func (c *RecorderConfig) defaults() {
	if len(c.QueueBuckets) == 0 {
		// Use bigger buckets than the default ones because the times of
		// waiting queues usually are greater than the handling, and resync
		// of events can be minutes.
		c.QueueBuckets = []float64{
			0.01, 0.05, 0.1, 0.25, 0.5, 1, 3, 10, 20, 60, 150, 300,
		}
	}

	if len(c.ProcessBuckets) == 0 {
		c.ProcessBuckets = []float64{
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
		}
	}
}

// DefaultRecorder implements the metrics recording in a prometheus registry.
type DefaultRecorder struct {
	// namespace is the prometheus metrics namespace.
	namespace string
	// subsystem is the prometheus metrics subsystem.
	subsystem string

	// reg is the prometheus registry.
	reg prometheus.Registerer

	// events counts the queued events.
	events *prometheus.CounterVec
	// queue measures the time an event stays in the queue.
	queue *prometheus.HistogramVec
	// process measures the time it takes to process an event.
	process *prometheus.HistogramVec
}

// NewRecorder returns a new Prometheus implementation for a metrics recorder.
func NewRecorder(
	config RecorderConfig, reg prometheus.Registerer,
) *DefaultRecorder {
	config.defaults()

	r := &DefaultRecorder{
		namespace: config.Namespace,
		subsystem: config.Subsystem,
		reg:       reg,

		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "queued_events_total",
			Help:      "Total number of events queued.",
		}, []string{"controller", "requeue"}),

		queue: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "event_in_queue_duration_seconds",
			Help:      "Duration of an event in the queue.",
			Buckets:   config.QueueBuckets,
		}, []string{"controller"}),

		process: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "processed_event_duration_seconds",
			Help:      "Duration of an event to be processed.",
			Buckets:   config.ProcessBuckets,
		}, []string{"controller", "success"}),
	}

	r.reg.MustRegister(r.events, r.queue, r.process)

	return r
}

// AddEvent satisfies controller.MetricsRecorder interface.
func (r *DefaultRecorder) AddEvent(
	_ context.Context, name string, isRequeue bool,
) {
	r.events.WithLabelValues(name, strconv.FormatBool(isRequeue)).Inc()
}

// GetEvent satisfies controller.MetricsRecorder interface.
//
// revive:disable-next-line:get-return // named after recorded event.
func (r *DefaultRecorder) GetEvent(
	_ context.Context, name string, queued time.Time,
) {
	r.queue.WithLabelValues(name).
		Observe(time.Since(queued).Seconds())
}

// DoneEvent satisfies controller.MetricsRecorder interface.
func (r *DefaultRecorder) DoneEvent(
	_ context.Context, name string, success bool, start time.Time,
) {
	r.process.WithLabelValues(name, strconv.FormatBool(success)).
		Observe(time.Since(start).Seconds())
}

// RegisterLen satisfies controller.MetricsRecorder interface.
func (r *DefaultRecorder) RegisterLen(
	controller string, call func(context.Context) int,
) {
	r.reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace:   r.namespace,
			Subsystem:   r.subsystem,
			Name:        "event_queue_length",
			Help:        "Length of the controller resource queue.",
			ConstLabels: prometheus.Labels{"controller": controller},
		},
		func() float64 { return float64(call(context.Background())) },
	))
}
