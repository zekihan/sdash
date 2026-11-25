package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/internal/kubeinfo"
)

func TestWriteActionsGating(t *testing.T) {
	paths := []string{
		"/api/actions/argo-sync",
		"/api/actions/restart",
		"/api/actions/proxmox-start",
		"/api/actions/wol",
		"/api/actions/homeassistant",
	}

	cfg := config.GetDefaultConfig()
	deps := new(Deps)
	deps.Config = cfg
	handler := New(deps).Handler()

	for _, path := range paths {
		rec := postAction(t, handler, path, `{}`)

		if rec.Code != http.StatusForbidden {
			t.Errorf(
				"POST %s with actions disabled = %d, want %d",
				path,
				rec.Code,
				http.StatusForbidden,
			)
		}
	}

	cfg.Actions.Enabled = true
	handler = New(deps).Handler()

	// Enabled but with no kube/proxmox/widgets clients configured.
	for _, path := range []string{
		"/api/actions/argo-sync", "/api/actions/restart",
		"/api/actions/proxmox-start", "/api/actions/homeassistant",
	} {
		rec := postAction(t, handler, path, `{}`)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf(
				"POST %s without client = %d, want %d",
				path,
				rec.Code,
				http.StatusServiceUnavailable,
			)
		}
	}

	rec := postAction(t, handler, "/api/actions/wol", `{"name":"nope"}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("POST wol unknown host = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = postAction(t, handler, "/api/actions/wol", `{`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST wol malformed body = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

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
			rec := postAction(t, handler, "/api/actions/argo-sync", tc.body)

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
