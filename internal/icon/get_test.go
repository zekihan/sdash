package icon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func testProxy(roundTrip roundTripFunc) *iconProxy {
	client := getHTTPClient()
	client.Transport = roundTrip

	return newIconProxy(client)
}

func iconRequest(t *testing.T, handler http.Handler, rawURL string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/api/proxy/icon?url="+url.QueryEscape(rawURL),
		http.NoBody,
	)
	request.Header.Set("Cookie", "session=fixture-only")
	request.Header.Set("Authorization", "Bearer fixture-only")
	request.Header.Set("X-Forwarded-For", "198.51.100.1")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestProxyValidatesAndCachesPNG(t *testing.T) {
	data := append(pngFixture(t), []byte("<script>bad</script>")...)
	calls := 0
	proxy := testProxy(func(request *http.Request) (*http.Response, error) {
		calls++

		if request.Header.Get("Cookie") != "" || request.Header.Get("Authorization") != "" {
			t.Error("inbound credentials forwarded")
		}

		response := imageResponse(data, "image/png")
		response.Header.Set("Set-Cookie", "bad=1")
		response.Header.Set("Content-Security-Policy", "unsafe")

		return response, nil
	})

	for range 2 {
		response := iconRequest(t, proxy, "https://public.example/icon.png")
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" ||
			response.Header().Get("X-Content-Type-Options") != "nosniff" ||
			response.Header().Get("Content-Security-Policy") != "default-src 'none'; sandbox" ||
			response.Header().Get("Cache-Control") != "public, max-age=600" ||
			response.Header().
				Get("Set-Cookie") !=
				"" || bytes.Contains(response.Body.Bytes(), []byte("<script>")) {
			t.Fatalf(
				"unsafe image response or cache replay: %d %v",
				response.Code,
				response.Header(),
			)
		}
	}

	if calls != 1 {
		t.Fatalf("expected cached validated PNG, got %d requests", calls)
	}
}

func TestProxyInvalidURL(t *testing.T) {
	proxy := testProxy(func(*http.Request) (*http.Response, error) {
		t.Error("invalid destination fetched")

		return nil, errTestDial
	})
	for _, raw := range []string{
		"", "invalid", "http://127.0.0.1/icon", "https://[::1]/icon",
		strings.Repeat("x", maxURLBytes+1),
	} {
		response := iconRequest(t, proxy, raw)
		if response.Code != http.StatusBadRequest ||
			response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("invalid URL got %d %v", response.Code, response.Header())
		}
	}
}

func TestProxyRejectsRedirectWithoutFollowing(t *testing.T) {
	calls := 0
	proxy := testProxy(func(*http.Request) (*http.Response, error) {
		calls++

		response := imageResponse([]byte("redirect body"), "text/plain")
		response.StatusCode = http.StatusFound
		response.Header.Set("Location", "http://169.254.169.254/latest/meta-data")

		return response, nil
	})

	response := iconRequest(t, proxy, "https://public.example/redirect")
	if response.Code != http.StatusBadGateway || calls != 1 || len(proxy.cache.entries) != 0 ||
		strings.Contains(
			response.Body.String(),
			"redirect body",
		) || response.Header().Get("Location") != "" {
		t.Fatal("followed, cached or disclosed redirect")
	}
}

func TestProxyDoesNotCacheRejectedResponses(t *testing.T) {
	for _, name := range []string{"HTML", "SVG", "oversize", "chunked oversize", "error"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			data := pngFixture(t)

			proxy := testProxy(func(*http.Request) (*http.Response, error) {
				calls++
				response := imageResponse(data, "image/png")

				switch name {
				case "HTML":
					response = imageResponse([]byte("<html>bad</html>"), "text/html")
				case "SVG":
					response = imageResponse(
						[]byte("<svg><script>bad</script></svg>"),
						"image/svg+xml",
					)
				case "oversize", "chunked oversize":
					response = imageResponse(make([]byte, maxIconBytes+1), "image/png")
					if name == "chunked oversize" {
						response.ContentLength = -1
					}
				case "error":
					response.StatusCode = http.StatusNotFound
				default:
					t.Fatal("unknown fixture")
				}

				return response, nil
			})
			for range 2 {
				response := iconRequest(t, proxy, "https://public.example/icon")
				if response.Code != http.StatusBadGateway ||
					response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("accepted rejected content: %d", response.Code)
				}
			}

			if calls != 2 || len(proxy.cache.entries) != 0 {
				t.Fatal("cached rejected response")
			}
		})
	}
}

func TestConcurrentIdenticalMissesCoalesce(t *testing.T) {
	data := pngFixture(t)
	started := make(chan struct{}, 1)

	release := make(chan struct{})
	releaseGate := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseGate)

	var calls atomic.Int32

	proxy := testProxy(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		started <- struct{}{}

		<-release

		return imageResponse(data, "image/png"), nil
	})

	const waiters = 20

	var workers sync.WaitGroup
	workers.Add(waiters)

	for range waiters {
		go func() {
			defer workers.Done()

			_, err := proxy.load(t.Context(), "https://public.example/icon")
			if err != nil {
				t.Errorf("load: %v", err)
			}
		}()
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("fetch did not start")
	}

	// A closed gate releases all current/future workers without introducing sleeps.
	releaseGate()

	workers.Wait()

	if calls.Load() != 1 {
		t.Fatalf("expected one coalesced fetch, got %d", calls.Load())
	}
}

func TestDistinctMissConcurrencyBudget(t *testing.T) {
	data := pngFixture(t)
	started := make(chan struct{}, maxConcurrentFetches)

	release := make(chan struct{})
	defer close(release)

	proxy := testProxy(func(*http.Request) (*http.Response, error) {
		started <- struct{}{}

		<-release

		return imageResponse(data, "image/png"), nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for index := range maxConcurrentFetches {
		go func() { _, _ = proxy.load(ctx, fmt.Sprintf("https://public.example/%d", index)) }()
	}

	for range maxConcurrentFetches {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("fetch did not start")
		}
	}

	response := iconRequest(t, proxy, "https://public.example/http-overflow")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP capacity rejection, got %d", response.Code)
	}

	_, err := proxy.load(t.Context(), "https://public.example/overflow")
	if !errors.Is(err, errBusy) {
		t.Fatalf("expected bounded concurrency rejection, got %v", err)
	}
}

func TestCallerCancellationDoesNotPoisonSharedFetch(t *testing.T) {
	data := pngFixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	proxy := testProxy(func(request *http.Request) (*http.Response, error) {
		close(started)

		select {
		case <-request.Context().Done():
			return nil, request.Context().Err()
		case <-release:
			return imageResponse(data, "image/png"), nil
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	first := make(chan error, 1)

	go func() { _, err := proxy.load(ctx, "https://public.example/icon"); first <- err }()

	<-started
	cancel()

	err := <-first
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected caller cancellation: %v", err)
	}

	close(release)

	_, err = proxy.load(t.Context(), "https://public.example/icon")
	if err != nil {
		t.Fatalf("shared fetch canceled: %v", err)
	}
}

func TestCallerRateBudget(t *testing.T) {
	proxy := newIconProxy(getHTTPClient())
	now := time.Now()

	proxy.now = func() time.Time { return now }
	for range requestsPerMinute {
		if !proxy.allowCaller("192.0.2.1:1234") {
			t.Fatal("rate budget exhausted early")
		}
	}

	if proxy.allowCaller("192.0.2.1:5678") || !proxy.allowCaller("192.0.2.2:1234") {
		t.Fatal("rate limit must group the connection peer independently of source port")
	}

	response := iconRequest(t, proxy, "https://public.example/icon")
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected rate rejection, got %d", response.Code)
	}

	now = now.Add(time.Minute)

	if !proxy.allowCaller("192.0.2.1:1234") {
		t.Fatal("expired caller window not released")
	}
}

func TestCallerTableIsBounded(t *testing.T) {
	proxy := newIconProxy(getHTTPClient())
	for index := range maxRateCallers {
		if !proxy.allowCaller(fmt.Sprintf("caller-%d", index)) {
			t.Fatal("caller table filled early")
		}
	}

	if proxy.allowCaller("overflow") || len(proxy.callers) != maxRateCallers {
		t.Fatal("caller table exceeded its bound")
	}
}
