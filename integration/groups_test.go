package integration

import (
	"bytes"
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
	p := filepath.Join(t.TempDir(), "dashboard.yaml")
	if err := os.WriteFile(p, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DASHBOARD_CONFIG_PATH", p)
	cfg := dashboard.GetConfigWithContext(t.Context())
	if cfg.Groups["custom"] == nil || !cfg.Groups["custom"].HasAny {
		t.Fatal("generated group missing or empty")
	}
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
	var normal bytes.Buffer
	if err := views.CardLists(cfg).Render(t.Context(), &normal); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	api.GetSearch(rr, httptest.NewRequest("GET", "/api/search?search=shared", nil))
	if rr.Code != 200 {
		t.Fatalf("search returned %d", rr.Code)
	}
	for _, id := range []string{"card-grid-custom", "card-first", "card-second"} {
		for name, html := range map[string]string{"dashboard": normal.String(), "search": rr.Body.String()} {
			if !strings.Contains(html, `id="`+id+`"`) {
				t.Errorf("%s missing %s", name, id)
			}
		}
	}
	for _, id := range []string{"card-grid-named", "card-explicit", "card-grid-others", "card-default"} {
		if !strings.Contains(normal.String(), `id="`+id+`"`) {
			t.Errorf("dashboard missing %s", id)
		}
		if strings.Contains(rr.Body.String(), `id="`+id+`"`) {
			t.Errorf("search unexpectedly contains %s", id)
		}
	}
}
