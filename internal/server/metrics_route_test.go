package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zekihan/go-commons/metrics"
	"github.com/zekihan/go-commons/middleware"
)

// TestMiddlewareRecordsMuxPattern serves a request through the production
// chain (registry.Middleware around recover and logging, as in main.go) and
// checks the recorded route is the mux pattern, not "unknown".
func TestMiddlewareRecordsMuxPattern(t *testing.T) {
	deps := new(Deps)
	deps.StartedAt = time.Now()
	srv := New(deps)

	registry := metrics.New()
	chain := registry.Middleware(
		middleware.RecoverHandler(middleware.Logging(srv.Handler())),
	)

	response := httptest.NewRecorder()
	chain.ServeHTTP(
		response,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/ping", http.NoBody),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("ping status = %d, want 200", response.Code)
	}

	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(
		recorder,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody),
	)

	body := recorder.Body.String()

	if !strings.Contains(
		body,
		`http_requests_total{method="GET",route="/api/ping",status="200"}`,
	) {
		t.Fatalf("metrics exposition missing mux pattern route:\n%s", body)
	}

	if strings.Contains(body, `route="unknown"`) {
		t.Fatalf("metrics exposition labels a request unknown:\n%s", body)
	}
}
