package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

func testMonitor(targets func() []*dashboard.Service) *Monitor {
	cfg := new(config.HealthConfig)
	cfg.Enabled = true
	cfg.Interval = time.Second
	cfg.Timeout = 2 * time.Second
	cfg.History = 3

	return New(*cfg, targets)
}

func testService(key, url string) *dashboard.Service {
	service := new(dashboard.Service)
	service.Key = key
	service.URL = url

	return service
}

func TestProbeStates(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("all good"))
	}))
	defer ok.Close()

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	keyword := testService("keyword", ok.URL)
	keyword.HealthKeyword = "good"

	keywordMissing := testService("keyword-missing", ok.URL)
	keywordMissing.HealthKeyword = "absent"

	disabled := testService("disabled", ok.URL)
	disabled.HealthDisabled = true

	services := []*dashboard.Service{
		testService("ok", ok.URL),
		keyword,
		keywordMissing,
		testService("broken", broken.URL),
		disabled,
	}

	monitor := testMonitor(func() []*dashboard.Service { return services })
	monitor.ProbeAll(t.Context())

	statuses := monitor.Snapshot()

	if got := statuses["ok"].State; got != StateUp {
		t.Errorf("ok state = %q, want up", got)
	}

	if got := statuses["keyword"].State; got != StateUp {
		t.Errorf("keyword state = %q, want up", got)
	}

	if got := statuses["keyword-missing"].State; got != StateDown {
		t.Errorf("keyword-missing state = %q, want down", got)
	}

	if got := statuses["broken"].State; got != StateDown {
		t.Errorf("broken state = %q, want down", got)
	}

	_, probed := statuses["disabled"]
	if probed {
		t.Error("healthDisabled service was probed")
	}

	if statuses["ok"].Uptime != 100 {
		t.Errorf("ok uptime = %v, want 100", statuses["ok"].Uptime)
	}
}

type delayedProbeTransport struct{}

func (delayedProbeTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	body, writer := io.Pipe()
	ready := time.After(slowLatency)

	go func() {
		<-ready

		_, _ = io.WriteString(writer, "all good")
		_ = writer.Close()
	}()

	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Body = body

	return response, nil
}

func TestProbeLatencyIncludesRequiredKeyword(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		statusOnly := testService("status-only", "http://service.example")
		keyword := testService("keyword", "http://service.example")
		keyword.HealthKeyword = "good"
		monitor := testMonitor(func() []*dashboard.Service {
			return []*dashboard.Service{statusOnly, keyword}
		})
		monitor.cfg.Timeout = 2 * slowLatency
		monitor.client.Timeout = monitor.cfg.Timeout
		monitor.client.Transport = delayedProbeTransport{}

		monitor.ProbeAll(t.Context())

		statuses := monitor.Snapshot()
		if got := statuses[statusOnly.Key]; got.State != StateUp || got.LatencyMs != 0 {
			t.Errorf("status-only probe = %s at %dms, want up at 0ms", got.State, got.LatencyMs)
		}

		if got := statuses[keyword.Key]; got.State != StateSlow ||
			got.LatencyMs != slowLatency.Milliseconds() {
			t.Errorf("keyword probe = %s at %dms, want slow at %dms",
				got.State, got.LatencyMs, slowLatency.Milliseconds())
		}
	})
}

func TestHistoryIsBoundedAndStaleTargetsDrop(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	services := []*dashboard.Service{testService("ok", ok.URL)}
	monitor := testMonitor(func() []*dashboard.Service { return services })

	for range 5 {
		monitor.ProbeAll(t.Context())
	}

	if got := len(monitor.Snapshot()["ok"].History); got != 3 {
		t.Fatalf("history length = %d, want cap of 3", got)
	}

	services = nil

	monitor.ProbeAll(t.Context())

	if got := len(monitor.Snapshot()); got != 0 {
		t.Fatalf("snapshot size = %d, want 0 after target removal", got)
	}
}

func TestUnreachableIsDown(t *testing.T) {
	services := []*dashboard.Service{testService("gone", "http://127.0.0.1:1")}
	monitor := testMonitor(func() []*dashboard.Service { return services })
	monitor.ProbeAll(t.Context())

	status := monitor.Snapshot()["gone"]
	if status.State != StateDown || status.Error == "" {
		t.Fatalf("state = %q error = %q, want down with error", status.State, status.Error)
	}
}
