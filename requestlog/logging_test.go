package requestlog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingExcludesCredentials(t *testing.T) {
	for _, target := range []string{
		"/user/oauth2/test/callback?code=code-secret&state=state-secret",
		"/login/test?redirect=redirect-secret&access_token=token-secret",
		"/api/proxy/icon?url=https%3A%2F%2Fusername-secret%3Apassword-secret%40example.com%2Ficon&apiKey=key-secret",
	} {
		for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError} {
			var logs bytes.Buffer

			original := slog.Default()

			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
			req.Header.Set("Authorization", "Bearer header-secret")
			req.Header.Set("Cookie", "auth=cookie-secret")

			handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery == "" {
					t.Error("logger removed handler query")
				}

				w.WriteHeader(status)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), req)
			slog.SetDefault(original)

			if strings.Contains(logs.String(), "secret") {
				t.Errorf("credentials in access log: %s", &logs)
			}

			var event map[string]any

			err := json.Unmarshal(logs.Bytes(), &event)
			if err != nil {
				t.Fatal(err)
			}

			if event["path"] != req.URL.Path || event["method"] != http.MethodGet ||
				event["status"] != float64(status) {
				t.Errorf("missing request metadata: %v", event)
			}

			for _, key := range []string{"size", "duration"} {
				if _, ok := event[key]; !ok {
					t.Errorf("missing %s", key)
				}
			}
		}
	}
}
