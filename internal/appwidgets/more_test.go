package appwidgets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

func integrationWith(url, apiKey string) config.IntegrationConfig {
	integration := new(config.IntegrationConfig)
	integration.URL = url
	integration.APIKey = apiKey

	return *integration
}

func TestQBittorrentWidget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			if r.FormValue("username") != "admin" || r.FormValue("password") != "pw" {
				w.WriteHeader(http.StatusForbidden)

				return
			}

			cookie := new(http.Cookie)
			cookie.Name = "SID"
			cookie.Value = "abc"
			cookie.HttpOnly = true
			cookie.Secure = true
			cookie.SameSite = http.SameSiteLaxMode
			http.SetCookie(w, cookie)
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/transfer/info":
			if r.Header.Get("Cookie") != "SID=abc" {
				w.WriteHeader(http.StatusForbidden)

				return
			}

			_, _ = w.Write([]byte(`{"dl_info_speed":5242880,"up_info_speed":1048576}`))
		case "/api/v2/torrents/info":
			_, _ = w.Write(
				[]byte(`[{"state":"downloading"},{"state":"uploading"},{"state":"pausedUP"}]`),
			)
		}
	}))
	defer server.Close()

	integration := new(config.IntegrationConfig)
	integration.URL = server.URL
	integration.Username = "admin"
	integration.Password = "pw"

	widget, err := New(new(config.IntegrationsConfig)).fetchQBittorrent(t.Context(), *integration)
	if err != nil {
		t.Fatalf("fetchQBittorrent: %v", err)
	}

	if widget.Stats[0].Value != "5.0 MB/s" || widget.Stats[2].Value != "2" {
		t.Errorf("stats = %+v, want 5.0 MB/s down and 2 active", widget.Stats)
	}
}

func TestMinifluxAndPaperless(t *testing.T) {
	miniflux := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"reads":{"1":2},"unreads":{"1":5,"2":7}}`))
	}))
	defer miniflux.Close()

	provider := New(new(config.IntegrationsConfig))

	widget, err := provider.fetchMiniflux(t.Context(), integrationWith(miniflux.URL, "k"))
	if err != nil || widget.Stats[0].Value != "12" {
		t.Fatalf("miniflux widget = %+v err=%v, want unread 12", widget, err)
	}

	paperless := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token pl-key" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"documents_total":412,"documents_inbox_total":3}`))
	}))
	defer paperless.Close()

	widget, err = provider.fetchPaperless(t.Context(), integrationWith(paperless.URL, "pl-key"))
	if err != nil || widget.Stats[0].Value != "412" || widget.Stats[1].Value != "3" {
		t.Fatalf("paperless widget = %+v err=%v, want 412 docs, 3 inbox", widget, err)
	}
}

func TestHomeAssistantWidget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"entity_id":"light.kitchen","state":"on","attributes":{"friendly_name":"Kitchen"}},
			{"entity_id":"light.hall","state":"off","attributes":{"friendly_name":"Hall"}},
			{"entity_id":"person.zeki","state":"home","attributes":{}},
			{"entity_id":"lock.front","state":"locked","attributes":{"friendly_name":"Front door"}}
		]`))
	}))
	defer server.Close()

	service := new(dashboard.Service)
	service.Widget = new(dashboard.Widget)
	service.Widget.Type = "homeassistant"
	service.Widget.Options = map[string]any{"entities": []any{"light.kitchen", "lock.front"}}

	provider := New(new(config.IntegrationsConfig))

	widget, err := provider.fetchHomeAssistant(
		t.Context(),
		service,
		integrationWith(server.URL, "ha-token"),
	)
	if err != nil {
		t.Fatalf("fetchHomeAssistant: %v", err)
	}

	if widget.Stats[0].Value != "1/2" || widget.Stats[1].Value != "1/1" {
		t.Errorf("stats = %+v, want lights 1/2 home 1/1", widget.Stats)
	}

	if len(widget.Controls) != 2 || !widget.Controls[0].On ||
		widget.Controls[0].Label != "Kitchen" {
		t.Errorf("controls = %+v, want Kitchen on + lock", widget.Controls)
	}
}
