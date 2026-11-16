package appwidgets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zekihan/sdash/config"
)

func TestCalendarMergesSonarrAndRadarr(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	sonarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(
			w,
			`[{"series":{"title":"Example Show"},"title":"Pilot",`+
				`"airDateUtc":%q,"seasonNumber":2,"episodeNumber":3,"hasFile":false}]`,
			tomorrow,
		)
	}))
	defer sonarr.Close()

	radarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w,
			`[{"title":"Example Movie","digitalRelease":%q,"year":2026,"hasFile":true}]`, tomorrow)
	}))
	defer radarr.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Sonarr = integrationWith(sonarr.URL, "k")
	integrations.Radarr = integrationWith(radarr.URL, "k")

	response := New(integrations).Calendar(t.Context())
	if !response.Available || len(response.Items) != 2 {
		t.Fatalf("calendar = %+v, want 2 items", response)
	}

	for _, item := range response.Items {
		if item.Source == "sonarr" && item.Detail != "S02E03 · Pilot" {
			t.Errorf("sonarr detail = %q", item.Detail)
		}
	}
}

func TestMediaRailAndLookupAdd(t *testing.T) {
	jellyfin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[{"id":"user-1"}]`))
		case "/Users/user-1/Items/Latest":
			_, _ = w.Write([]byte(
				`[{"id":"item-1","name":"Pilot","seriesName":"Example Show","parentIndexNumber":1,"indexNumber":1}]`,
			))
		case "/Items/item-1/Images/Primary":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpegbytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer jellyfin.Close()

	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(
			`[{"type":"on_this_day","data":{"year":3},"assets":[{"id":"asset-1","type":"IMAGE"}]}]`,
		))
	}))
	defer immich.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Jellyfin = integrationWith(jellyfin.URL, "k")
	integrations.Immich = integrationWith(immich.URL, "k")

	provider := New(integrations)
	rail := provider.MediaRail(t.Context())

	if len(rail.Jellyfin) != 1 || rail.Jellyfin[0].Title != "Example Show" {
		t.Fatalf("jellyfin rail = %+v", rail.Jellyfin)
	}

	if len(rail.Immich) != 1 || rail.Immich[0].Title != "3 years ago" {
		t.Fatalf("immich rail = %+v", rail.Immich)
	}

	content, contentType, err := provider.JellyfinImage(t.Context(), "item-1")
	if err != nil || contentType != "image/jpeg" || string(content) != "jpegbytes" {
		t.Fatalf("image proxy = %q %q err=%v", content, contentType, err)
	}

	_, _, traversalErr := provider.JellyfinImage(t.Context(), "../etc")
	if traversalErr == nil {
		t.Fatal("path traversal id accepted")
	}
}

func TestArrLookupAndAddMovie(t *testing.T) {
	added := false

	radarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/movie/lookup":
			_, _ = w.Write(
				[]byte(`[{"title":"Example Movie","tmdbId":42,"year":2026,"overview":"o"}]`),
			)
		case r.URL.Path == "/api/v3/movie/lookup/tmdb":
			_, _ = w.Write([]byte(`{"title":"Example Movie","tmdbId":42,"year":2026}`))
		case r.URL.Path == "/api/v3/qualityprofile":
			_, _ = w.Write([]byte(`[{"id":7}]`))
		case r.URL.Path == "/api/v3/rootfolder":
			_, _ = w.Write([]byte(`[{"path":"/movies"}]`))
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost:
			added = true

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer radarr.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Radarr = integrationWith(radarr.URL, "k")
	provider := New(integrations)

	results, err := provider.Lookup(t.Context(), "radarr", "Example")
	if err != nil || len(results) != 1 || results[0].ExternalID != 42 {
		t.Fatalf("lookup = %+v err=%v", results, err)
	}

	err = provider.AddTitle(t.Context(), "radarr", 42)
	if err != nil || !added {
		t.Fatalf("add err=%v added=%v", err, added)
	}
}
