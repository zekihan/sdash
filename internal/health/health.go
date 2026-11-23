// Package health probes the configured dashboard services on an interval and
// keeps a bounded per-service history for status dots, latency sparklines and
// uptime percentages. Probe targets come only from the operator's
// dashboard.yaml, never from request input, and private addresses are the
// normal case in a homelab.
package health

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zekihan/go-commons/metrics"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

const (
	maxProbeBodyBytes   = 64 << 10
	maxRedirects        = 5
	probeWorkers        = 8
	dialTimeout         = 3 * time.Second
	slowLatency         = 2 * time.Second
	maxIdleConnsPerHost = 2
	maxErrorLength      = 160
	percentScale        = 100.0
)

// State is the health verdict for one probe.
type State string

const (
	StateUp      State = "up"
	StateDown    State = "down"
	StateSlow    State = "slow"
	StateUnknown State = "unknown"
)

// Sample is one probe result.
type Sample struct {
	At      time.Time
	State   State
	Error   string
	Latency time.Duration
	Code    int
}

// SampleJSON is the wire form of a Sample with latency in milliseconds.
type SampleJSON struct {
	At      time.Time `json:"at"`
	State   State     `json:"state"`
	Error   string    `json:"error,omitempty"`
	Latency int64     `json:"latencyMs"`
	Code    int       `json:"code"`
}

func (sample Sample) wire() SampleJSON {
	wire := new(SampleJSON)
	wire.At = sample.At
	wire.State = sample.State
	wire.Error = sample.Error
	wire.Latency = sample.Latency.Milliseconds()
	wire.Code = sample.Code

	return *wire
}

// Status is the aggregated health view of one service.
type Status struct {
	CheckedAt time.Time    `json:"checkedAt"`
	Key       string       `json:"key"`
	Error     string       `json:"error,omitempty"`
	State     State        `json:"state"`
	History   []SampleJSON `json:"history"`
	LatencyMs int64        `json:"latencyMs"`
	Uptime    float64      `json:"uptime"`
	Code      int          `json:"code"`
}

// ChangeListener receives the full status map after every probe round.
type ChangeListener func(map[string]*Status)

type entry struct {
	samples []Sample
}

type probeResult struct {
	key    string
	sample Sample
}

// Monitor runs the probe loop.
type Monitor struct {
	client    *http.Client
	insecure  *http.Client
	targets   func() []*dashboard.Service
	entries   map[string]*entry
	metrics   *metrics.Registry
	listeners []ChangeListener
	cfg       config.HealthConfig
	mu        sync.RWMutex
	probing   sync.Mutex
}

// New creates a Monitor. Targets supplies the current service list on every
// round so dashboard.yaml reloads are picked up without a restart.
func New(cfg config.HealthConfig, targets func() []*dashboard.Service) *Monitor {
	dialer := new(net.Dialer)
	dialer.Timeout = dialTimeout

	base := new(http.Transport)
	base.Proxy = http.ProxyFromEnvironment
	base.DialContext = dialer.DialContext
	base.MaxIdleConnsPerHost = maxIdleConnsPerHost

	tlsConfig := new(tls.Config)
	tlsConfig.InsecureSkipVerify = true

	insecureTransport := base.Clone()
	insecureTransport.TLSClientConfig = tlsConfig

	monitor := new(Monitor)
	monitor.cfg = cfg
	monitor.client = newProbeClient(cfg.Timeout, base)
	monitor.insecure = newProbeClient(cfg.Timeout, insecureTransport)
	monitor.targets = targets
	monitor.entries = make(map[string]*entry)

	return monitor
}

func newProbeClient(timeout time.Duration, transport *http.Transport) *http.Client {
	client := new(http.Client)
	client.Transport = transport
	client.Timeout = timeout
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return http.ErrUseLastResponse
		}

		return nil
	}

	return client
}

// OnChange registers a listener invoked after each probe round.
func (monitor *Monitor) OnChange(listener ChangeListener) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	monitor.listeners = append(monitor.listeners, listener)
}

// Run probes immediately, then on the configured interval, until ctx ends.
func (monitor *Monitor) Run(ctx context.Context) {
	if !monitor.cfg.Enabled {
		return
	}

	monitor.ProbeAll(ctx)

	ticker := time.NewTicker(monitor.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.ProbeAll(ctx)
		}
	}
}

// ProbeAll probes every enabled target concurrently and notifies listeners.
// Overlapping rounds are skipped.
func (monitor *Monitor) ProbeAll(ctx context.Context) {
	if !monitor.probing.TryLock() {
		return
	}
	defer monitor.probing.Unlock()

	targets := monitor.enabledTargets()
	results := monitor.runWorkers(ctx, targets)
	listeners := monitor.record(targets, results)

	if len(listeners) > 0 {
		snapshot := monitor.Snapshot()
		for _, listener := range listeners {
			listener(snapshot)
		}
	}
}

// Snapshot returns the aggregated status of every probed service.
func (monitor *Monitor) Snapshot() map[string]*Status {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()

	out := make(map[string]*Status, len(monitor.entries))
	for key, record := range monitor.entries {
		out[key] = aggregate(key, record.samples)
	}

	return out
}

func (monitor *Monitor) enabledTargets() []*dashboard.Service {
	targets := make([]*dashboard.Service, 0)

	for _, service := range monitor.targets() {
		if service == nil || service.HealthDisabled || service.ProbeURL() == "" {
			continue
		}

		targets = append(targets, service)
	}

	return targets
}

func (monitor *Monitor) runWorkers(
	ctx context.Context,
	targets []*dashboard.Service,
) []probeResult {
	jobs := make(chan *dashboard.Service)
	found := make(chan probeResult, len(targets))

	var waitGroup sync.WaitGroup
	for range probeWorkers {
		waitGroup.Go(func() {
			for service := range jobs {
				result := new(probeResult)
				result.key = service.Key

				result.sample = monitor.probe(ctx, service)
				found <- *result
			}
		})
	}

	for _, service := range targets {
		jobs <- service
	}

	close(jobs)
	waitGroup.Wait()
	close(found)

	results := make([]probeResult, 0, len(targets))
	for result := range found {
		results = append(results, result)
	}

	return results
}

func (monitor *Monitor) record(
	targets []*dashboard.Service,
	results []probeResult,
) []ChangeListener {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	live := make(map[string]bool, len(targets))
	for _, service := range targets {
		live[service.Key] = true
	}

	for _, result := range results {
		record, ok := monitor.entries[result.key]
		if !ok {
			record = new(entry)
			monitor.entries[result.key] = record
		}

		record.samples = append(record.samples, result.sample)
		if overflow := len(record.samples) - max(1, monitor.cfg.History); overflow > 0 {
			record.samples = record.samples[overflow:]
		}
	}

	for key := range monitor.entries {
		if !live[key] {
			delete(monitor.entries, key)
		}
	}

	monitor.observeLocked(results)

	listeners := make([]ChangeListener, len(monitor.listeners))
	copy(listeners, monitor.listeners)

	return listeners
}

func (monitor *Monitor) probe(ctx context.Context, service *dashboard.Service) Sample {
	sample := new(Sample)
	sample.At = time.Now()
	sample.State = StateDown

	reqCtx, cancel := context.WithTimeout(ctx, monitor.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, service.ProbeURL(), http.NoBody)
	if err != nil {
		sample.Error = "invalid URL"

		return *sample
	}

	req.Header.Set("User-Agent", "sdash-health/1.0")

	client := monitor.client
	if service.HealthInsecure {
		client = monitor.insecure
	}

	start := time.Now()

	resp, err := client.Do(req)
	sample.Latency = time.Since(start)

	if err != nil {
		sample.Error = trimError(err)

		return *sample
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			slog.DebugContext(ctx, "Probe body close failed", slog.Any("error", closeErr))
		}
	}()

	sample.Code = resp.StatusCode
	if !service.IsHealthyStatus(sample.Code) {
		sample.Error = resp.Status
		drain(resp)

		return *sample
	}

	if service.HealthKeyword == "" {
		drain(resp)
	} else {
		sample.Error = checkKeyword(resp.Body, service.HealthKeyword)
		sample.Latency = time.Since(start)
	}

	switch {
	case sample.Error != "":
		return *sample
	case sample.Latency >= slowLatency:
		sample.State = StateSlow
	default:
		sample.State = StateUp
	}

	return *sample
}

func checkKeyword(reader io.Reader, keyword string) string {
	body, err := io.ReadAll(io.LimitReader(reader, maxProbeBodyBytes))
	if err != nil {
		return "failed to read body"
	}

	if !strings.Contains(string(body), keyword) {
		return "keyword not found"
	}

	return ""
}

func drain(resp *http.Response) {
	_, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxProbeBodyBytes))
	if err != nil {
		slog.Debug("Probe body drain failed", slog.Any("error", err))
	}
}

func trimError(err error) string {
	msg := err.Error()
	if len(msg) > maxErrorLength {
		msg = msg[:maxErrorLength]
	}

	return msg
}

func aggregate(key string, samples []Sample) *Status {
	status := new(Status)
	status.Key = key
	status.State = StateUnknown
	status.History = make([]SampleJSON, 0, len(samples))

	if len(samples) == 0 {
		return status
	}

	up := 0

	for _, sample := range samples {
		status.History = append(status.History, sample.wire())
		if sample.State == StateUp || sample.State == StateSlow {
			up++
		}
	}

	last := samples[len(samples)-1]
	status.State = last.State
	status.CheckedAt = last.At
	status.LatencyMs = last.Latency.Milliseconds()
	status.Code = last.Code
	status.Error = last.Error
	status.Uptime = float64(up) / float64(len(samples)) * percentScale

	return status
}
