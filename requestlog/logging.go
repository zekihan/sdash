// Package requestlog provides credential-safe HTTP access logs.
package requestlog

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
)

// Logging records an allowlist of request metadata. Query strings, full URLs,
// headers and bodies are deliberately excluded, including on authentication errors.
func Logging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := httpsnoop.CaptureMetrics(handler, w, r)
		slog.InfoContext(r.Context(), "HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", metrics.Code),
			slog.Int64("size", metrics.Written),
			slog.Duration("duration", metrics.Duration.Round(time.Millisecond)),
		)
	})
}
