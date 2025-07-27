package errorroutes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet5xx(t *testing.T) {
	tests := []struct {
		name           string
		acceptType     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "HTML response",
			acceptType:     "text/html",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "<!doctype html>",
		},
		{
			name:           "Default response",
			acceptType:     "application/json",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/error", nil)
			req.Header.Set("Accept", tt.acceptType)
			rec := httptest.NewRecorder()

			Get5xx(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, res.StatusCode)
			}

			body := rec.Body.String()
			if len(tt.expectedBody) > 0 && body[:len(tt.expectedBody)] != tt.expectedBody {
				t.Errorf("expected body to start with %q, got %q", tt.expectedBody, body)
			}
		})
	}
}
