package viewroutes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoot(t *testing.T) {
	tests := []struct {
		name           string
		urlPath        string
		acceptType     string
		expectedHeader string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "Root path redirects to /dashboard",
			urlPath:        "/",
			acceptType:     "text/html",
			expectedStatus: http.StatusMovedPermanently,
			expectedHeader: "/dashboard",
			expectedBody:   "",
		},
		{
			name:           "Non-root path with HTML accept type redirects to /404",
			urlPath:        "/nonexistent",
			acceptType:     "text/html",
			expectedStatus: http.StatusMovedPermanently,
			expectedHeader: "/404",
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				tt.urlPath,
				http.NoBody,
			)
			req.Header.Set("Accept", tt.acceptType)

			rr := httptest.NewRecorder()

			Root(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedHeader != "" {
				location := rr.Header().Get("Location")
				if location != tt.expectedHeader {
					t.Errorf("expected Location header %q, got %q", tt.expectedHeader, location)
				}
			}

			if tt.expectedBody != "" {
				body := rr.Body.String()
				if body != tt.expectedBody {
					t.Errorf("expected body %q, got %q", tt.expectedBody, body)
				}
			}
		})
	}
}
