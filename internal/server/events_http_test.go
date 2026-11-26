package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/internal/health"
)

func sseClient(t *testing.T, url string) (*http.Response, *bufio.Reader, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}

	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp, bufio.NewReader(resp.Body), cancel
}

// readSSEEvent reads one event: block (event:/data: lines) from the stream.
func readSSEEvent(t *testing.T, reader *bufio.Reader) (string, string) {
	t.Helper()

	type result struct {
		err   error
		event string
		data  string
	}

	done := make(chan result, 1)

	go func() {
		var eventType, data string

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- result{event: eventType, data: data, err: err}

				return
			}

			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if eventType != "" || data != "" {
					done <- result{event: eventType, data: data, err: nil}

					return
				}

				continue
			}

			if strings.HasPrefix(line, ":") {
				continue // heartbeat comment
			}

			if after, ok := strings.CutPrefix(line, "event: "); ok {
				eventType = after
			}

			if after, ok := strings.CutPrefix(line, "data: "); ok {
				data = after
			}
		}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf(
				"read SSE event: %v (partial event=%q data=%q)",
				result.err,
				result.event,
				result.data,
			)
		}

		return result.event, result.data
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SSE event")

		return "", ""
	}
}

func hubClientCount(srv *Server) int {
	srv.hub.mu.RLock()
	defer srv.hub.mu.RUnlock()

	return len(srv.hub.clients)
}

func TestSSEConfigAndHealthEventsReachClient(t *testing.T) {
	cfg := config.GetDefaultConfig()
	deps := actionDeps(cfg)
	deps.StartedAt = time.Now()
	deps.Monitor = health.New(
		config.HealthConfig{
			Enabled:  true,
			History:  10,
			Interval: time.Second,
			Timeout:  time.Second,
		},
		func() []*dashboard.Service { return nil },
	)
	srv := New(deps)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, reader, _ := sseClient(t, ts.URL+"/api/events")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want 200", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("events Content-Type = %q, want text/event-stream", ct)
	}

	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("events Cache-Control = %q, want no-cache", cc)
	}

	// Initial health snapshot is pushed on connect.
	eventType, data := readSSEEvent(t, reader)
	if eventType != "health" {
		t.Errorf("initial event = %q, want health", eventType)
	}

	_ = data

	// Config-reload notification.
	srv.NotifyConfig(nil)

	eventType, data = readSSEEvent(t, reader)
	if eventType != "config" || data != `{"reloaded":true}` {
		t.Errorf("config event = %q %q, want config {\"reloaded\":true}", eventType, data)
	}

	// Health notification carries the status map.
	status := new(health.Status)
	status.Key = "web"
	status.State = health.StateUp
	status.Code = 200
	srv.NotifyHealth(map[string]*health.Status{"web": status})

	eventType, data = readSSEEvent(t, reader)
	if eventType != "health" || !strings.Contains(data, `"web"`) ||
		!strings.Contains(data, `"up"`) {
		t.Errorf("health event = %q %q, want health containing web/up", eventType, data)
	}
}

func TestSSEDisconnectCleansUpHubSubscription(t *testing.T) {
	cfg := config.GetDefaultConfig()
	deps := actionDeps(cfg)
	deps.StartedAt = time.Now()
	srv := New(deps)

	// Drive the real handler with a cancellable request context: canceling
	// it is exactly what the HTTP server does when a browser disconnects.
	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", http.NoBody)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)

		srv.handleEvents(rec, req)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for hubClientCount(srv) != 1 && time.Now().Before(deadline) {
		runtime.Gosched()
	}

	if got := hubClientCount(srv); got != 1 {
		t.Fatalf("hub clients while connected = %d, want 1", got)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SSE handler did not return after disconnect")
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hubClientCount(srv) == 0 {
			break
		}

		runtime.Gosched()
	}

	if got := hubClientCount(srv); got != 0 {
		t.Fatalf("hub clients after disconnect = %d, want 0 (subscription leaked)", got)
	}

	// Broadcasting after the last client left must not block or panic.
	broadcastDone := make(chan struct{})
	go func() {
		defer close(broadcastDone)

		srv.NotifyConfig(nil)
		srv.NotifyHealth(map[string]*health.Status{})
	}()

	select {
	case <-broadcastDone:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked after disconnect")
	}
}

func TestShutdownEndsEventStreams(t *testing.T) {
	cfg := config.GetDefaultConfig()
	deps := actionDeps(cfg)
	deps.Monitor = health.New(cfg.Health, func() []*dashboard.Service { return nil })
	srv := New(deps)

	ts := httptest.NewUnstartedServer(srv.Handler())
	ts.Config.RegisterOnShutdown(srv.CloseEvents)
	ts.Start()
	t.Cleanup(ts.Close)

	resp, reader, cancelClient := sseClient(t, ts.URL+"/api/events")
	defer cancelClient()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want 200", resp.StatusCode)
	}

	// Confirm the stream is active before starting shutdown.
	eventType, _ := readSSEEvent(t, reader)
	if eventType != "health" {
		t.Fatalf("initial event = %q, want health", eventType)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := ts.Config.Shutdown(ctx)
	elapsed := time.Since(start)
	t.Logf("Shutdown returned %v after %s", err, elapsed.Round(time.Millisecond))

	if err != nil {
		t.Fatalf("shutdown with an open event stream: %v", err)
	}

	if elapsed >= time.Second {
		t.Errorf("shutdown took %s, want under 1s", elapsed)
	}

	_, err = reader.ReadByte()
	if !errors.Is(err, io.EOF) {
		t.Errorf("reading ended event stream: got %v, want EOF", err)
	}
}

func TestSSESlowClientDoesNotBlockBroadcast(t *testing.T) {
	hub := newHub()

	slow := hub.subscribe() // never drained
	defer hub.unsubscribe(slow)

	done := make(chan struct{})
	go func() {
		defer close(done)

		for i := range sseBufferSize * 4 {
			hub.broadcast(event{Type: "config", Data: map[string]int{"i": i}})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on a slow client")
	}

	if len(slow) != sseBufferSize {
		t.Errorf(
			"slow client buffer = %d, want capped at %d (drop, not grow)",
			len(slow),
			sseBufferSize,
		)
	}

	// A draining client still receives events in order (FIFO for what fits).
	fast := hub.subscribe()
	defer hub.unsubscribe(fast)

	hub.broadcast(event{Type: "health", Data: map[string]bool{"ok": true}})

	select {
	case ev := <-fast:
		if ev.Type != "health" {
			t.Errorf("event type = %q, want health", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("draining client received no event")
	}
}
