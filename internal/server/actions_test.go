package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/internal/kubeinfo"
)

func TestArgoSyncRejectsInvalidRequest(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true

	deps := new(Deps)
	deps.Config = cfg
	deps.Kube = new(kubeinfo.Client)
	handler := New(deps).Handler()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed JSON", body: `{"app":`, want: "invalid request body"},
		{name: "missing app", body: `{}`, want: "app is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/api/actions/argo-sync",
				strings.NewReader(tc.body),
			)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, request)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			var response map[string]string

			err := json.Unmarshal(rec.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("decode response %q as a single JSON object: %v", rec.Body.String(), err)
			}

			if got := response["error"]; got != tc.want {
				t.Errorf("error = %q, want %q", got, tc.want)
			}
		})
	}
}
