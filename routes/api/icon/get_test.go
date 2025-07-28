package icon

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zekihan/go-commons/web"
)

func TestProxy(t *testing.T) {
	// Initialize cache for tests
	InitIconCache()

	t.Run("missing url parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/icon", http.NoBody)
		w := httptest.NewRecorder()

		Proxy(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("successful proxy request", func(t *testing.T) {
		// Create mock server
		mockServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusOK)
				web.WriteString(w, "fake-icon-data")
			}),
		)
		defer mockServer.Close()

		req := httptest.NewRequest(http.MethodGet, "/icon?url="+mockServer.URL, http.NoBody)
		rr := httptest.NewRecorder()

		Proxy(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		if rr.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", rr.Header().Get("Content-Type"))
		}

		if rr.Header().Get("Cache-Control") != "public, max-age=86400" {
			t.Errorf("expected Cache-Control header to be set")
		}

		if rr.Body.String() != "fake-icon-data" {
			t.Errorf("expected body 'fake-icon-data', got %s", rr.Body.String())
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		// Add item to cache first
		testURL := "http://example.com/icon.png"
		cache.add(testURL, &Icon{
			URL:         testURL,
			Data:        []byte("cached-icon-data"),
			ContentType: "image/png",
		})

		req := httptest.NewRequest(http.MethodGet, "/icon?url="+testURL, http.NoBody)
		rr := httptest.NewRecorder()

		Proxy(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		if rr.Body.String() != "cached-icon-data" {
			t.Errorf("expected cached data, got %s", rr.Body.String())
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/icon?url=invalid-url", http.NoBody)
		w := httptest.NewRecorder()

		Proxy(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestGetHTTPClient(t *testing.T) {
	client := getHTTPClient()

	if client.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if transport.MaxIdleConns != 10 {
		t.Errorf("expected MaxIdleConns 10, got %d", transport.MaxIdleConns)
	}

	if transport.IdleConnTimeout != 30*time.Second {
		t.Errorf("expected IdleConnTimeout 30s, got %v", transport.IdleConnTimeout)
	}
}
