package views

import (
	"bytes"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config/dashboard"
)

func TestGroupLinksPreserveKeys(t *testing.T) {
	for _, key := range []string{
		"monitoring", "ops#prod", "ops?region=eu", "ops/tools",
		"ops%tools", "ops&tools", "İzleme araçları",
	} {
		t.Run(key, func(t *testing.T) {
			group := new(dashboard.Group)
			group.Key = key
			group.Title = "Tools"
			group.HasAny = true
			cfg := new(dashboard.Config)
			cfg.GroupList = dashboard.GroupList{group}

			var rendered bytes.Buffer

			err := CardLists(cfg).Render(t.Context(), &rendered)
			if err != nil {
				t.Fatal(err)
			}

			_, link, found := strings.Cut(rendered.String(), `href="`)
			if !found {
				t.Fatalf("missing group link in %s", rendered.String())
			}

			target, _, found := strings.Cut(link, `"`)
			if !found {
				t.Fatal("unterminated group link")
			}

			target = html.UnescapeString(target)
			// Go's router must receive the original key as one path segment.
			mux := http.NewServeMux()
			mux.HandleFunc(
				"/dashboard/group/{group}",
				func(w http.ResponseWriter, r *http.Request) {
					if got := r.PathValue("group"); got != key {
						t.Errorf("route received key %q, want %q", got, key)
					}

					w.WriteHeader(http.StatusNoContent)
				},
			)

			response := httptest.NewRecorder()
			mux.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody),
			)

			if response.Code != http.StatusNoContent {
				t.Errorf("group link returned %d", response.Code)
			}
		})
	}
}
