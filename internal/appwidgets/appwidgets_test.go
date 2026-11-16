package appwidgets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

func serviceWithWidget(key, widgetType string) *dashboard.Service {
	service := new(dashboard.Service)
	service.Key = key
	service.Widget = new(dashboard.Widget)
	service.Widget.Type = widgetType

	return service
}

func TestArrAndJellyfinWidgets(t *testing.T) {
	arr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		switch r.URL.Path {
		case "/api/v3/queue":
			_, _ = w.Write([]byte(`{"totalRecords":3}`))
		case "/api/v3/wanted/missing":
			_, _ = w.Write([]byte(`{"totalRecords":12}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer arr.Close()

	jellyfin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"UserName":"zeki","NowPlayingItem":{"Name":"Pilot","SeriesName":"Example Show","RunTimeTicks":1000},
			 "PlayState":{"PositionTicks":250}},
			{"UserName":"idle"}
		]`))
	}))
	defer jellyfin.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Sonarr.URL = arr.URL
	integrations.Sonarr.APIKey = "test-key"
	integrations.Jellyfin.URL = jellyfin.URL
	integrations.Jellyfin.APIKey = "jf-key"

	provider := New(integrations)
	widgets := provider.ForServices(t.Context(), []*dashboard.Service{
		serviceWithWidget("sonarr", "sonarr"),
		serviceWithWidget("sonarr-local", "sonarr"),
		serviceWithWidget("jellyfin", "jellyfin"),
		serviceWithWidget("radarr", "radarr"), // not configured: must be absent
	})

	sonarr := widgets["sonarr"]
	if sonarr == nil || len(sonarr.Stats) != 2 || sonarr.Stats[0].Value != "3" ||
		sonarr.Stats[1].Value != "12" {
		t.Fatalf("sonarr widget = %+v, want queue 3 missing 12", sonarr)
	}

	if widgets["sonarr-local"] == nil || widgets["sonarr-local"].Stats[0].Value != "3" {
		t.Errorf("sonarr-local widget = %+v, want same stats", widgets["sonarr-local"])
	}

	jf := widgets["jellyfin"]
	if jf == nil || len(jf.Lines) != 1 || jf.Lines[0] != "Example Show — Pilot (25%) · zeki" {
		t.Fatalf("jellyfin widget = %+v, want one now-playing line", jf)
	}

	if _, present := widgets["radarr"]; present {
		t.Error("radarr widget present without configuration")
	}
}
