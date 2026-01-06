package controller_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
)

// TODO: this is an AI generated test that needs to be reviewed and improved.

type newRecorderParams struct {
	config controller.RecorderConfig
}

var newRecorderTestCases = map[string]newRecorderParams{
	"default-config": {
		config: controller.RecorderConfig{},
	},

	"with-namespace": {
		config: controller.RecorderConfig{
			Namespace: "test_namespace",
		},
	},

	"with-subsystem": {
		config: controller.RecorderConfig{
			Subsystem: "test_subsystem",
		},
	},

	"with-namespace-and-subsystem": {
		config: controller.RecorderConfig{
			Namespace: "app",
			Subsystem: "controller",
		},
	},

	"with-custom-buckets": {
		config: controller.RecorderConfig{
			Namespace:      "custom",
			Subsystem:      "sub",
			QueueBuckets:   []float64{1, 5, 10},
			ProcessBuckets: []float64{0.1, 1},
		},
	},
}

func TestNewRecorder(t *testing.T) {
	test.Map(t, newRecorderTestCases).
		Run(func(t test.Test, param newRecorderParams) {
			// Given
			reg := prometheus.NewRegistry()

			// When
			recorder := controller.NewRecorder(param.config, reg)

			// Then
			assert.NotNil(t, recorder)
		})
}

type addEventParams struct {
	name    string
	requeue bool
}

var addEventTestCases = map[string]addEventParams{
	"add-initial-event": {
		name:    "test-controller",
		requeue: false,
	},

	"add-requeue-event": {
		name:    "test-controller",
		requeue: true,
	},

	"multiple-events": {
		name:    "multi-controller",
		requeue: false,
	},
}

func TestAddEvent(t *testing.T) {
	test.Map(t, addEventTestCases).
		Run(func(t test.Test, param addEventParams) {
			// Given
			reg := prometheus.NewRegistry()
			recorder := controller.NewRecorder(
				controller.RecorderConfig{}, reg)
			ctx := context.Background()

			// When
			recorder.AddEvent(ctx, param.name, param.requeue)

			// Then
			metrics := getMetricFamilies(t, reg)
			counter := findMetric(t, metrics, "queued_events_total",
				param.name, strconv.FormatBool(param.requeue))
			assert.Equal(t, 1.0, counter.GetCounter().GetValue())
		})
}

type getEventParams struct {
	name      string
	queued    time.Time
	expectMin float64
}

var getEventTestCases = map[string]getEventParams{
	"immediate-get": {
		name:      "test-controller",
		queued:    time.Now(),
		expectMin: 0.0,
	},

	"delayed-get": {
		name:      "delayed-controller",
		queued:    time.Now().Add(-100 * time.Millisecond),
		expectMin: 0.09,
	},

	"old-event": {
		name:      "old-controller",
		queued:    time.Now().Add(-5 * time.Second),
		expectMin: 4.9,
	},
}

func TestGetEvent(t *testing.T) {
	test.Map(t, getEventTestCases).
		Run(func(t test.Test, param getEventParams) {
			// Given
			reg := prometheus.NewRegistry()
			recorder := controller.NewRecorder(
				controller.RecorderConfig{}, reg)
			ctx := context.Background()

			// When
			recorder.GetEvent(ctx, param.name, param.queued)

			// Then
			metrics := getMetricFamilies(t, reg)
			histogram := findHistogram(
				t, metrics, "event_in_queue_duration_seconds", param.name)
			assert.Equal(t, uint64(1), histogram.GetSampleCount())
			assert.Greater(t, histogram.GetSampleSum(), param.expectMin)
		})
}

type doneEventParams struct {
	name      string
	success   bool
	start     time.Time
	expectMin float64
}

var doneEventTestCases = map[string]doneEventParams{
	"success-fast": {
		name:      "fast-controller",
		success:   true,
		start:     time.Now(),
		expectMin: 0.0,
	},

	"success-slow": {
		name:      "slow-controller",
		success:   true,
		start:     time.Now().Add(-2 * time.Second),
		expectMin: 1.9,
	},

	"failure-fast": {
		name:      "fail-controller",
		success:   false,
		start:     time.Now(),
		expectMin: 0.0,
	},

	"failure-slow": {
		name:      "fail-slow-controller",
		success:   false,
		start:     time.Now().Add(-3 * time.Second),
		expectMin: 2.9,
	},
}

func TestDoneEvent(t *testing.T) {
	test.Map(t, doneEventTestCases).
		Run(func(t test.Test, param doneEventParams) {
			// Given
			reg := prometheus.NewRegistry()
			recorder := controller.NewRecorder(
				controller.RecorderConfig{}, reg)
			ctx := context.Background()

			// When
			recorder.DoneEvent(ctx, param.name, param.success, param.start)

			// Then
			metrics := getMetricFamilies(t, reg)
			histogram := findHistogram(
				t, metrics, "processed_event_duration_seconds",
				param.name, strconv.FormatBool(param.success))
			assert.Equal(t, uint64(1), histogram.GetSampleCount())
			assert.Greater(t, histogram.GetSampleSum(), param.expectMin)
		})
}

type registerLenParams struct {
	controller string
	call       func(context.Context) int
	expectLen  int
}

var registerLenTestCases = map[string]registerLenParams{
	"zero-length": {
		controller: "empty-controller",
		call: func(context.Context) int {
			return 0
		},
		expectLen: 0,
	},

	"positive-length": {
		controller: "queue-controller",
		call: func(context.Context) int {
			return 5
		},
		expectLen: 5,
	},

	"large-length": {
		controller: "large-controller",
		call: func(context.Context) int {
			return 1000
		},
		expectLen: 1000,
	},
}

func TestRegisterLen(t *testing.T) {
	test.Map(t, registerLenTestCases).
		Run(func(t test.Test, param registerLenParams) {
			// Given
			reg := prometheus.NewRegistry()
			recorder := controller.NewRecorder(
				controller.RecorderConfig{}, reg)

			// When
			recorder.RegisterLen(param.controller, param.call)

			// Then
			metrics := getMetricFamilies(t, reg)
			gauge := findGauge(t, metrics, "event_queue_length",
				param.controller)
			assert.Equal(t, float64(param.expectLen), gauge.GetGauge().GetValue())
		})
}

// Helper functions for metric extraction.

func getMetricFamilies(
	t test.Test, reg *prometheus.Registry,
) []*dto.MetricFamily {
	metrics, err := reg.Gather()
	require.NoError(t, err)

	return metrics
}

func findMetric(
	t test.Test, families []*dto.MetricFamily,
	name string, labels ...string,
) *dto.Metric {
	for _, family := range families {
		if family.GetName() == name {
			for _, metric := range family.GetMetric() {
				if matchLabels(metric, labels...) {
					return metric
				}
			}
		}
	}
	require.Fail(t, "metric not found", "name=%s, labels=%v", name, labels)

	return nil
}

func findHistogram(
	t test.Test, families []*dto.MetricFamily,
	name string, labels ...string,
) *dto.Histogram {
	metric := findMetric(t, families, name, labels...)

	return metric.GetHistogram()
}

func findGauge(
	t test.Test, families []*dto.MetricFamily,
	name string, controller string,
) *dto.Metric {
	for _, family := range families {
		if family.GetName() == name {
			for _, metric := range family.GetMetric() {
				for _, label := range metric.GetLabel() {
					if label.GetName() == "controller" &&
						label.GetValue() == controller {
						return metric
					}
				}
			}
		}
	}
	require.Fail(t, "gauge metric not found",
		"name=%s, controller=%s", name, controller)

	return nil
}

func matchLabels(metric *dto.Metric, labelValues ...string) bool {
	labels := metric.GetLabel()
	if len(labels) != len(labelValues) {
		return false
	}

	for i, label := range labels {
		if label.GetValue() != labelValues[i] {
			return false
		}
	}

	return true
}
