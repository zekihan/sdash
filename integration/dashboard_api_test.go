package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/internal/health"
	"github.com/zekihan/sdash/internal/server"
)

const fixture = `
orderGroupsVia: key
settings:
  title: Test Lab
services:
  first: {title: Shared First, group: custom, url: "https://example.com/first", description: "First service"}
  second: {title: Shared Second, group: custom, url: "https://example.com/second"}
  explicit: {title: Explicit Service, group: named, url: "https://example.com/named"}
  default: {title: Default Service, url: "https://example.com/default"}
groups:
  named: {title: Named Group, order: 4}
`

type dashboardPayload struct {
	Settings struct {
		Title string `json:"title"`
	} `json:"settings"`
	Groups []struct {
		Key      string `json:"key"`
		Title    string `json:"title"`
		Services []struct {
			Key         string `json:"key"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Group       string `json:"group"`
		} `json:"services"`
	} `json:"groups"`
	Features struct {
		Health bool `json:"health"`
	} `json:"features"`
}

func buildHandler(t *testing.T) http.Handler {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "dashboard.yaml")

	err := os.WriteFile(configPath, []byte(fixture), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DASHBOARD_CONFIG_PATH", configPath)

	cfg := dashboard.GetConfigWithContext(t.Context())
	if cfg.Groups["custom"] == nil || !cfg.Groups["custom"].HasAny {
		t.Fatal("generated group missing or empty")
	}

	appCfg := config.GetConfigWithContext(t.Context())
	monitor := health.New(appCfg.Health, func() []*dashboard.Service {
		return dashboard.GetConfigWithContext(t.Context()).ServicesList
	})

	deps := new(server.Deps)
	deps.Config = appCfg
	deps.Monitor = monitor
	deps.StartedAt = time.Now()

	return server.New(deps).Handler()
}

func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody),
	)

	return rec
}

func TestDashboardAPI(t *testing.T) {
	handler := buildHandler(t)

	t.Run("dashboard payload", func(t *testing.T) {
		rec := get(t, handler, "/api/dashboard")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		payload := new(dashboardPayload)

		err := json.Unmarshal(rec.Body.Bytes(), payload)
		if err != nil {
			t.Fatalf("decode payload: %v", err)
		}

		checkPayload(t, payload)
	})

	t.Run("me reports auth disabled", func(t *testing.T) {
		rec := get(t, handler, "/api/me")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		var me struct {
			AuthEnabled   bool `json:"authEnabled"`
			Authenticated bool `json:"authenticated"`
		}

		err := json.Unmarshal(rec.Body.Bytes(), &me)
		if err != nil {
			t.Fatal(err)
		}

		if me.AuthEnabled || !me.Authenticated {
			t.Errorf("me = %+v, want auth disabled and authenticated", me)
		}
	})

	t.Run("unknown api route is a JSON 404", func(t *testing.T) {
		rec := get(t, handler, "/api/nope")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

func checkPayload(t *testing.T, payload *dashboardPayload) {
	t.Helper()

	if payload.Settings.Title != "Test Lab" {
		t.Errorf("settings.title = %q, want Test Lab", payload.Settings.Title)
	}

	if !payload.Features.Health {
		t.Error("features.health = false, want true (default)")
	}

	titles := map[string][]string{}

	for _, group := range payload.Groups {
		for _, service := range group.Services {
			titles[group.Key] = append(titles[group.Key], service.Title)
			if service.Group != group.Key {
				t.Errorf(
					"service %q group = %q inside group %q",
					service.Key,
					service.Group,
					group.Key,
				)
			}
		}
	}

	if len(titles["custom"]) != 2 || len(titles["named"]) != 1 || len(titles["others"]) != 1 {
		t.Errorf("group membership = %v, want custom:2 named:1 others:1", titles)
	}
}
