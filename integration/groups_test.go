package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes/api"
	"github.com/zekihan/sdash/views"
)

func TestGeneratedGroupsRenderAndSearch(t *testing.T) {
	fixture := `
orderGroupsVia: key
services:
  first: {title: Shared First, group: custom, url: "https://example.com/first"}
  second: {title: Shared Second, group: custom, url: "https://example.com/second"}
  explicit: {title: Explicit Service, group: named, url: "https://example.com/named"}
  default: {title: Default Service, url: "https://example.com/default"}
groups:
  named: {title: Named Group, order: 4}
`

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

	checkGeneratedGroup(t, cfg)

	var normal bytes.Buffer

	err = views.CardLists(cfg).Render(t.Context(), &normal)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	api.GetSearch(
		rr,
		httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/search?search=shared",
			http.NoBody,
		),
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("search returned %d", rr.Code)
	}

	checkRenderedGroups(t, normal.String(), rr.Body.String())
}

func checkGeneratedGroup(t *testing.T, cfg *dashboard.Config) {
	t.Helper()

	count := 0

	for _, group := range cfg.GroupList {
		if group.Key == "custom" {
			count++

			if group != cfg.Groups["custom"] {
				t.Error("group list differs from map")
			}
		}
	}

	if count != 1 {
		t.Fatalf("generated group appears %d times", count)
	}
}

func checkRenderedGroups(t *testing.T, normal, search string) {
	t.Helper()

	for _, id := range []string{"card-grid-custom", "card-first", "card-second"} {
		for name, html := range map[string]string{"dashboard": normal, "search": search} {
			if !strings.Contains(html, `id="`+id+`"`) {
				t.Errorf("%s missing %s", name, id)
			}
		}
	}

	for _, id := range []string{"card-grid-named", "card-explicit", "card-grid-others", "card-default"} {
		if !strings.Contains(normal, `id="`+id+`"`) {
			t.Errorf("dashboard missing %s", id)
		}

		if strings.Contains(search, `id="`+id+`"`) {
			t.Errorf("search unexpectedly contains %s", id)
		}
	}
}
