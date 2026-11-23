package health

import (
	"github.com/zekihan/go-commons/metrics"
)

// ObserveMetrics attaches a registry for bounded health signals: per-round
// probe outcomes and current per-state service counts. States come from the
// fixed State set; no service names are recorded.
func (monitor *Monitor) ObserveMetrics(registry *metrics.Registry) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	monitor.metrics = registry
}

// observeLocked records one probe round. Callers must hold monitor.mu.
func (monitor *Monitor) observeLocked(results []probeResult) {
	if monitor.metrics == nil {
		return
	}

	for _, result := range results {
		monitor.metrics.Counter("sdash_health_probe_total", metrics.Labels{
			"outcome": string(result.sample.State),
		}).Inc()
	}

	counts := map[State]int{
		StateUp:      0,
		StateDown:    0,
		StateSlow:    0,
		StateUnknown: 0,
	}

	for _, record := range monitor.entries {
		if len(record.samples) == 0 {
			continue
		}

		state := record.samples[len(record.samples)-1].State
		if _, ok := counts[state]; ok {
			counts[state]++
		}
	}

	for state, count := range counts {
		monitor.metrics.Gauge("sdash_health_services", metrics.Labels{
			"state": string(state),
		}).Set(float64(count))
	}
}
