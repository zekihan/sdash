package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/go-commons/metrics"

	"github.com/zekihan/sdash/config/dashboard"
)

func TestObserveMetricsRecordsProbeRound(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("all good"))
	}))
	defer ok.Close()

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	services := []*dashboard.Service{
		testService("ok", ok.URL),
		testService("broken", broken.URL),
	}

	monitor := testMonitor(func() []*dashboard.Service { return services })
	registry := metrics.New()
	monitor.ObserveMetrics(registry)
	monitor.ProbeAll(t.Context())

	if got := registry.Counter("sdash_health_probe_total", metrics.Labels{
		"outcome": "up",
	}).Get(); got != 1 {
		t.Fatalf("up probes = %d, want 1", got)
	}

	if got := registry.Counter("sdash_health_probe_total", metrics.Labels{
		"outcome": "down",
	}).Get(); got != 1 {
		t.Fatalf("down probes = %d, want 1", got)
	}

	if got := registry.Gauge("sdash_health_services", metrics.Labels{
		"state": "up",
	}).Get(); got != 1 {
		t.Fatalf("services up = %v, want 1", got)
	}

	if got := registry.Gauge("sdash_health_services", metrics.Labels{
		"state": "down",
	}).Get(); got != 1 {
		t.Fatalf("services down = %v, want 1", got)
	}
}
