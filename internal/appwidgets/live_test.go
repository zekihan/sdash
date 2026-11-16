package appwidgets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

func TestQBittorrent5LoginCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			cookie := new(http.Cookie)
			cookie.Name = "QBT_SID_8090"
			cookie.Value = "abc"
			cookie.HttpOnly = true
			cookie.Secure = true
			cookie.SameSite = http.SameSiteLaxMode
			http.SetCookie(w, cookie)
			w.WriteHeader(http.StatusNoContent)
		case "/api/v2/transfer/info":
			if r.Header.Get("Cookie") != "QBT_SID_8090=abc" {
				w.WriteHeader(http.StatusForbidden)

				return
			}

			_, _ = w.Write([]byte(`{"dl_info_speed":0,"up_info_speed":0}`))
		case "/api/v2/torrents/info":
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	integration := new(config.IntegrationConfig)
	integration.URL = server.URL
	integration.Username = "admin"
	integration.Password = "pw"

	_, err := New(new(config.IntegrationsConfig)).fetchQBittorrent(t.Context(), *integration)
	if err != nil {
		t.Fatalf("fetchQBittorrent: %v", err)
	}
}

func TestQBittorrentWithoutCredentialsSkipsLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			t.Error("login must not be attempted without credentials")
		case "/api/v2/transfer/info":
			_, _ = w.Write([]byte(`{"dl_info_speed":0,"up_info_speed":0}`))
		case "/api/v2/torrents/info":
			_, _ = w.Write([]byte(`[{"state":"downloading"}]`))
		}
	}))
	defer server.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.QBittorrent.URL = server.URL
	provider := New(integrations)

	integration, ready := provider.integrationFor("qbittorrent")
	if !ready || !provider.Enabled() {
		t.Fatal("qBittorrent with only a URL should be ready")
	}

	widget, err := provider.fetchQBittorrent(t.Context(), integration)
	if err != nil {
		t.Fatalf("fetchQBittorrent: %v", err)
	}

	if widget.Stats[2].Value != "1" {
		t.Errorf("active = %q, want 1", widget.Stats[2].Value)
	}
}

func TestImmichSendsOnlyTheAPIKeyHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "key" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"images":3,"videos":1,"total":4}`))
	}))
	defer server.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Immich = integrationWith(server.URL, "key")
	provider := New(integrations)

	widget, err := provider.fetchImmich(t.Context(), provider.integrations.Immich)
	if err != nil {
		t.Fatalf("fetchImmich: %v", err)
	}

	if widget.Stats[0].Value != "3" {
		t.Errorf("photos = %q, want 3", widget.Stats[0].Value)
	}
}

func TestJellyfinUsesMediaBrowserAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != `MediaBrowser Token="key"` {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Jellyfin = integrationWith(server.URL, "key")
	provider := New(integrations)

	_, err := provider.fetchJellyfin(t.Context(), provider.integrations.Jellyfin)
	if err != nil {
		t.Fatalf("fetchJellyfin: %v", err)
	}
}

func TestSemaphoreCapsRecentTasks(t *testing.T) {
	tasks := make([]string, 0, 25)
	for range 25 {
		tasks = append(tasks, `{"id":1,"status":"success","template_id":1}`)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/tasks"):
			_, _ = w.Write([]byte("[" + strings.Join(tasks, ",") + "]"))
		case strings.HasSuffix(r.URL.Path, "/templates"):
			_, _ = w.Write([]byte(`[{"id":1,"name":"deploy"}]`))
		case r.URL.Path == "/api/projects":
			_, _ = w.Write([]byte(`[{"id":1}]`))
		}
	}))
	defer server.Close()

	provider := New(new(config.IntegrationsConfig))
	service := serviceWithWidget("semaphore", "semaphore")

	widget, err := provider.fetchSemaphore(t.Context(), service, integrationWith(server.URL, "key"))
	if err != nil {
		t.Fatalf("fetchSemaphore: %v", err)
	}

	if widget.Stats[0].Value != "10/10" {
		t.Errorf("recent ok = %q, want 10/10", widget.Stats[0].Value)
	}
}

func TestServiceWidgetsNeverEncodeNullLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"totalRecords":0}`))
	}))
	defer server.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Sonarr = integrationWith(server.URL, "key")

	widgets := New(integrations).ForServices(
		t.Context(),
		[]*dashboard.Service{serviceWithWidget("sonarr", "sonarr")},
	)

	encoded, err := json.Marshal(widgets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), "null") {
		t.Errorf("widgets encode null lists: %s", encoded)
	}
}
