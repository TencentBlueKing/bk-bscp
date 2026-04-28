package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var taskMetricNames = map[string]struct{}{
	"step_running_count":            {},
	"step_execute_total":            {},
	"step_execute_duration_seconds": {},
	"task_execute_total":            {},
	"task_execute_duration_seconds": {},
}

type metricFamilyFilterGatherer struct {
	gatherer prometheus.Gatherer
	names    map[string]struct{}
}

func (g metricFamilyFilterGatherer) Gather() ([]*dto.MetricFamily, error) {
	if g.gatherer == nil {
		return nil, nil
	}

	mfs, err := g.gatherer.Gather()
	filtered := make([]*dto.MetricFamily, 0, len(g.names))
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		if _, ok := g.names[mf.GetName()]; ok {
			filtered = append(filtered, mf)
		}
	}

	return filtered, err
}

func taskMetricGatherer() prometheus.Gatherer {
	return metricFamilyFilterGatherer{
		gatherer: prometheus.DefaultGatherer,
		names:    taskMetricNames,
	}
}
