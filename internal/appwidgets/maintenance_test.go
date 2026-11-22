package appwidgets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestForgejoAndSyncthingEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/notifications":
			if r.URL.Query().Get("status-types") != "unread" {
				t.Error("missing unread filter")
			}

			_, _ = w.Write([]byte(`[{},{ } ]`))
		case "/api/v1/issues/search":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/rest/db/completion":
			_, _ = w.Write([]byte(`{"completion":97.6}`))
		case "/rest/system/connections":
			_, _ = w.Write(
				[]byte(`{"connections":{"a":{"connected":true},"b":{"connected":false}}}`),
			)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := New(new(config.IntegrationsConfig))
	integration := integrationWith(server.URL, "key")

	forgejo, err := provider.fetchForgejo(t.Context(), integration)
	if err != nil || len(forgejo.Stats) != 1 || forgejo.Stats[0].Value != "2" {
		t.Fatalf("forgejo = %+v, %v", forgejo, err)
	}

	sync, err := provider.fetchSyncthing(t.Context(), integration)
	if err != nil || len(sync.Stats) != 2 || sync.Stats[0].Value != "98%" ||
		sync.Stats[1].Value != "1/2" {
		t.Fatalf("syncthing = %+v, %v", sync, err)
	}
}

func TestToggleHomeAssistant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/homeassistant/toggle" ||
			r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf(
				"unexpected request: %s %s authorization %q",
				r.Method,
				r.URL.Path,
				r.Header.Get("Authorization"),
			)
		}

		var body map[string]any

		err := json.NewDecoder(r.Body).
			Decode(&body)
		if err != nil ||
			body["entity_id"] != "light.desk" {
			t.Errorf("body = %v, %v", body, err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := new(config.IntegrationsConfig)
	cfg.HomeAssistant = integrationWith(server.URL, "key")

	provider := New(cfg)

	err := provider.ToggleHomeAssistant(t.Context(), "light.desk")
	if err != nil {
		t.Fatal(err)
	}

	err = New(
		new(config.IntegrationsConfig),
	).ToggleHomeAssistant(t.Context(), "light.desk")
	if err == nil {
		t.Fatal("expected unconfigured error")
	}
}
