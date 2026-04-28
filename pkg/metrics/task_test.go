package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricFamilyFilterGathererKeepsOnlyConfiguredMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	taskMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "task_execute_total",
		Help: "task metric",
	})
	otherMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines",
		Help: "non-task metric",
	})
	reg.MustRegister(taskMetric, otherMetric)

	gatherer := metricFamilyFilterGatherer{
		gatherer: reg,
		names: map[string]struct{}{
			"task_execute_total": {},
		},
	}
	mfs, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics failed: %v", err)
	}

	if len(mfs) != 1 {
		t.Fatalf("expected one filtered metric family, got %d", len(mfs))
	}
	if got := mfs[0].GetName(); got != "task_execute_total" {
		t.Fatalf("expected task_execute_total, got %s", got)
	}
}

func TestTaskMetricNamesIncludeFrameworkCollectors(t *testing.T) {
	for _, name := range []string{
		"step_running_count",
		"step_execute_total",
		"step_execute_duration_seconds",
		"task_execute_total",
		"task_execute_duration_seconds",
	} {
		if _, ok := taskMetricNames[name]; !ok {
			t.Fatalf("expected task metric %s to be registered for exposure", name)
		}
	}
}
