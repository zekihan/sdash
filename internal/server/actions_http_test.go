package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/internal/appwidgets"
	"github.com/zekihan/sdash/internal/kubeinfo"
	"github.com/zekihan/sdash/internal/proxmoxinfo"
	"github.com/zekihan/sdash/session/auth"
)

var allActionPaths = []string{
	"/api/actions/argo-sync",
	"/api/actions/restart",
	"/api/actions/proxmox-start",
	"/api/actions/wol",
	"/api/actions/homeassistant",
	"/api/actions/qbittorrent-add",
	"/api/actions/arr-add",
}

func postAction(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		path,
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func expectActionError(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
	wantErr string,
) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, wantStatus, rec.Body.String())
	}

	var body map[string]string

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}

	if wantErr != "" && body["error"] != wantErr {
		t.Errorf("error = %q, want %q", body["error"], wantErr)
	}
}

// --- fake backends ---------------------------------------------------------

type recordedRequest struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

type recorder struct {
	reqs []recordedRequest
	mu   sync.Mutex
}

func (r *recorder) add(req *http.Request) {
	body, _ := io.ReadAll(req.Body)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.reqs = append(r.reqs, recordedRequest{
		Method: req.Method, Path: req.URL.RequestURI(), Body: string(body),
		Auth: req.Header.Get("Authorization"),
	})
}

func (r *recorder) all() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]recordedRequest(nil), r.reqs...)
}

func fakeKube(t *testing.T, status int) (*kubeinfo.Client, *recorder) {
	t.Helper()

	rec := new(recorder)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.WriteHeader(status)
	}))
	t.Cleanup(backend.Close)

	kubeconfig := "apiVersion: v1\nkind: Config\ncurrent-context: test\n" +
		"clusters:\n- name: test\n  cluster:\n    server: " + backend.URL + "\n" +
		"contexts:\n- name: test\n  context:\n    cluster: test\n    user: test\n" +
		"users:\n- name: test\n  user:\n    token: test-token\n"

	path := filepath.Join(t.TempDir(), "kubeconfig")

	writeErr := os.WriteFile(path, []byte(kubeconfig), 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	kubeCfg := new(config.KubernetesConfig)
	kubeCfg.Kubeconfig = path
	kubeCfg.ArgocdNamespace = "argocd"

	client, err := kubeinfo.New(*kubeCfg)
	if err != nil {
		t.Fatalf("kubeinfo.New: %v", err)
	}

	return client, rec
}

func actionDeps(cfg *config.Config) *Deps {
	deps := new(Deps)
	deps.Config = cfg

	return deps
}

func TestCrossOriginWritesRejected(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = true

	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	cfg.WebConfig.SessionTimeout = time.Hour
	deps := actionDeps(cfg)
	kube, backend := fakeKube(t, http.StatusOK)
	deps.Kube = kube
	prefsPath := filepath.Join(t.TempDir(), "prefs.json")
	deps.Prefs = NewPreferenceStore(prefsPath)
	handler := New(deps).Handler()
	sessionCookie := mintSessionCookie(t, cfg)

	for _, path := range allActionPaths {
		t.Run(strings.TrimPrefix(path, "/api/actions/"), func(t *testing.T) {
			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "https://dashboard.example.test"+path, http.NoBody,
			)
			req.Header.Set("Origin", "https://sibling.example.test")
			req.Header.Set("Sec-Fetch-Site", "same-site")
			req.Header.Set("Content-Type", "text/plain")
			req.AddCookie(sessionCookie)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
			}
		})
	}

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPut, "https://dashboard.example.test/api/preferences", http.NoBody,
	)
	req.Header.Set("Origin", "https://sibling.example.test")
	req.AddCookie(sessionCookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("preferences status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}

	_, err := os.Stat(prefsPath)
	if !os.IsNotExist(err) {
		t.Errorf("preferences file should be absent, stat error = %v", err)
	}

	if calls := len(backend.all()); calls != 0 {
		t.Errorf("backend calls = %d, want 0", calls)
	}
}

func TestCrossOriginActionAllowsSameOriginJSON(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = true

	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	cfg.WebConfig.SessionTimeout = time.Hour
	deps := actionDeps(cfg)
	kube, backend := fakeKube(t, http.StatusOK)
	deps.Kube = kube
	handler := New(deps).Handler()
	sessionCookie := mintSessionCookie(t, cfg)

	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
	}{
		{
			name:        "plain text",
			contentType: "text/plain",
			body:        `{}`,
			status:      http.StatusUnsupportedMediaType,
		},
		{
			name:        "missing type",
			contentType: "",
			body:        `{}`,
			status:      http.StatusUnsupportedMediaType,
		},
		{
			name:        "JSON",
			contentType: "application/json",
			body:        `{"app":"test-app"}`,
			status:      http.StatusOK,
		},
		{
			name: "JSON with charset", contentType: "application/json; charset=utf-8",
			body: `{"app":"test-app"}`, status: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"https://dashboard.example.test/api/actions/argo-sync",
				strings.NewReader(tc.body),
			)
			req.Header.Set("Origin", "https://dashboard.example.test")
			req.Header.Set("Sec-Fetch-Site", "same-origin")

			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			req.AddCookie(sessionCookie)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
		})
	}

	if calls := len(backend.all()); calls != 2 {
		t.Errorf("backend calls = %d, want 2", calls)
	}
}

// --- gating ----------------------------------------------------------------

func TestAllActionsForbiddenWhenDisabled(t *testing.T) {
	cfg := config.GetDefaultConfig() // Actions disabled by default.
	handler := New(actionDeps(cfg)).Handler()

	for _, path := range allActionPaths {
		// Even a malformed body must not bypass the gate: 403 wins over 400.
		for _, body := range []string{`{}`, `{`} {
			rec := postAction(t, handler, path, body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("POST %s body=%q disabled = %d, want 403", path, body, rec.Code)
			}

			expectErrContains(t, rec, "actions are disabled")
		}
	}
}

func expectErrContains(t *testing.T, rec *httptest.ResponseRecorder, substr string) {
	t.Helper()

	var body map[string]string

	err := json.Unmarshal(
		rec.Body.Bytes(),
		&body,
	)
	if err != nil ||
		!strings.Contains(body["error"], substr) {
		t.Errorf("body = %q, want error containing %q", rec.Body.String(), substr)
	}
}

func TestActionsEnabledWithoutBackends(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	handler := New(actionDeps(cfg)).Handler()

	for _, path := range []string{
		"/api/actions/argo-sync", "/api/actions/restart", "/api/actions/proxmox-start",
		"/api/actions/homeassistant", "/api/actions/qbittorrent-add", "/api/actions/arr-add",
	} {
		expectActionError(t, postAction(t, handler, path, `{}`), http.StatusServiceUnavailable, "")
	}
	// WOL has no backend client; unknown host (incl. empty name) is a 404.
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/wol", `{}`),
		http.StatusNotFound,
		"unknown wake host",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/wol", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
}

// --- validation + fake-backend success/failure ------------------------------

func TestArgoSyncAndRestartValidationAndBackend(t *testing.T) {
	kube, rec := fakeKube(t, http.StatusOK)
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	deps := actionDeps(cfg)
	deps.Kube = kube
	handler := New(deps).Handler()

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/argo-sync", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/argo-sync", `{}`),
		http.StatusBadRequest,
		"app is required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/argo-sync", `{"app":""}`),
		http.StatusBadRequest,
		"app is required",
	)

	if rec2 := postAction(
		t, handler,
		"/api/actions/argo-sync",
		`{"app":"my-app"}`,
	); rec2.Code != http.StatusOK {
		t.Fatalf("argo-sync valid = %d, want 200 (%s)", rec2.Code, rec2.Body.String())
	}

	got := rec.all()
	if len(got) != 1 || got[0].Method != http.MethodPatch ||
		got[0].Path != "/apis/argoproj.io/v1alpha1/namespaces/argocd/applications/my-app" ||
		!strings.Contains(got[0].Body, `"sync"`) {
		t.Errorf("kube backend requests = %+v, want one PATCH sync for my-app", got)
	}

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/restart", `{}`),
		http.StatusBadRequest,
		"namespace and name are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/restart", `{"namespace":"ns"}`),
		http.StatusBadRequest,
		"namespace and name are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/restart", `{"name":"web"}`),
		http.StatusBadRequest,
		"namespace and name are required",
	)

	if rec2 := postAction(
		t, handler,
		"/api/actions/restart",
		`{"namespace":"apps","name":"web"}`,
	); rec2.Code != http.StatusOK {
		t.Fatalf("restart valid = %d, want 200", rec2.Code)
	}

	got = rec.all()

	last := got[len(got)-1]
	if last.Method != http.MethodPatch ||
		last.Path != "/apis/apps/v1/namespaces/apps/deployments/web" {
		t.Errorf("restart backend request = %+v", last)
	}
}

func TestArgoSyncBackendFailureIsBadGateway(t *testing.T) {
	kube, _ := fakeKube(t, http.StatusInternalServerError)
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	deps := actionDeps(cfg)
	deps.Kube = kube
	handler := New(deps).Handler()

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/argo-sync", `{"app":"x"}`),
		http.StatusBadGateway,
		"sync failed",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/restart", `{"namespace":"a","name":"b"}`),
		http.StatusBadGateway,
		"restart failed",
	)
}

func TestProxmoxStartValidationAndBackend(t *testing.T) {
	rec := new(recorder)
	status := http.StatusOK

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.WriteHeader(status)
	}))
	defer backend.Close()

	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	deps := actionDeps(cfg)
	proxCfg := new(config.ProxmoxConfig)
	proxCfg.URL = backend.URL
	proxCfg.APIToken = "user@pam!tok=secret"
	deps.Proxmox = proxmoxinfo.New(*proxCfg)
	handler := New(deps).Handler()

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{}`),
		http.StatusBadRequest,
		"node and vmid are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{"node":"pve1"}`),
		http.StatusBadRequest,
		"node and vmid are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{"node":"pve1","vmid":0}`),
		http.StatusBadRequest,
		"node and vmid are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{"node":"pve1","vmid":-3}`),
		http.StatusBadRequest,
		"node and vmid are required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/proxmox-start", `{"vmid":100}`),
		http.StatusBadRequest,
		"invalid node",
	)
	expectActionError(
		t,
		postAction(
			t,
			handler,
			"/api/actions/proxmox-start",
			`{"node":"pve_1","kind":"qemu","vmid":100}`,
		),
		http.StatusBadRequest,
		"invalid node",
	)

	if len(rec.all()) != 0 {
		t.Fatal("invalid request reached the proxmox backend")
	}

	if r := postAction(
		t, handler,
		"/api/actions/proxmox-start",
		`{"node":"pve1","kind":"qemu","vmid":100}`,
	); r.Code != http.StatusOK {
		t.Fatalf("proxmox-start valid = %d, want 200 (%s)", r.Code, r.Body.String())
	}

	got := rec.all()
	if len(got) != 1 || got[0].Method != http.MethodPost ||
		got[0].Path != "/api2/json/nodes/pve1/qemu/100/status/start" {
		t.Errorf("proxmox backend = %+v, want start POST for qemu/100", got)
	}

	// Unsupported kind is rejected by the client before any HTTP call.
	before := len(rec.all())

	expectActionError(
		t,
		postAction(
			t, handler,
			"/api/actions/proxmox-start",
			`{"node":"pve1","kind":"bogus","vmid":100}`,
		),
		http.StatusBadGateway,
		"start failed",
	)

	if len(rec.all()) != before {
		t.Error("unsupported kind reached the proxmox backend")
	}

	status = http.StatusInternalServerError

	expectActionError(
		t,
		postAction(
			t, handler,
			"/api/actions/proxmox-start",
			`{"node":"pve1","kind":"lxc","vmid":101}`,
		),
		http.StatusBadGateway,
		"start failed",
	)
}

func TestWakeOnLanValidationAndSend(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	cfg.Actions.WakeOnLan = []config.WakeHost{
		{Name: "nas", MAC: "00:11:22:33:44:55", Broadcast: "127.0.0.1"},
		{Name: "broken", MAC: "not-a-mac", Broadcast: "127.0.0.1"},
	}
	handler := New(actionDeps(cfg)).Handler()

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/wol", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/wol", `{"name":"nope"}`),
		http.StatusNotFound,
		"unknown wake host",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/wol", `{"name":"broken"}`),
		http.StatusBadGateway,
		"wake-on-lan failed",
	)

	if r := postAction(t, handler, "/api/actions/wol", `{"name":"nas"}`); r.Code != http.StatusOK {
		t.Errorf("wol valid host = %d, want 200 (%s)", r.Code, r.Body.String())
	}
}

func widgetDeps(t *testing.T, integrations *config.IntegrationsConfig) http.Handler {
	t.Helper()

	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	deps := actionDeps(cfg)
	deps.Widgets = appwidgets.New(integrations)

	return New(deps).Handler()
}

func TestHomeAssistantActionValidationAndBackend(t *testing.T) {
	rec := new(recorder)
	status := http.StatusOK

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		rec.mu.Lock()
		rec.reqs = append(
			rec.reqs,
			recordedRequest{
				Method: r.Method,
				Path:   r.URL.Path,
				Body:   string(body),
				Auth:   r.Header.Get("Authorization"),
			},
		)
		rec.mu.Unlock()
		w.WriteHeader(status)
	}))
	defer backend.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.HomeAssistant.URL = backend.URL
	integrations.HomeAssistant.APIKey = "ha-key"
	handler := widgetDeps(t, integrations)

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/homeassistant", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/homeassistant", `{}`),
		http.StatusBadRequest,
		"entityId is required",
	)

	if r := postAction(
		t, handler,
		"/api/actions/homeassistant",
		`{"entityId":"light.desk"}`,
	); r.Code != http.StatusOK {
		t.Fatalf("homeassistant valid = %d, want 200 (%s)", r.Code, r.Body.String())
	}

	got := rec.all()
	if len(got) != 1 || got[0].Path != "/api/services/homeassistant/toggle" ||
		got[0].Auth != "Bearer ha-key" ||
		!strings.Contains(got[0].Body, "light.desk") {
		t.Errorf("HA backend = %+v", got)
	}

	status = http.StatusInternalServerError

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/homeassistant", `{"entityId":"light.desk"}`),
		http.StatusBadGateway,
		"toggle failed",
	)
}

func TestQBittorrentAddValidationAndBackend(t *testing.T) {
	var sawAdd bool

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			sid := new(http.Cookie)
			sid.Name = "SID"
			sid.Value = "abc"
			sid.Path = "/"
			sid.Secure = true
			sid.HttpOnly = true
			sid.SameSite = http.SameSiteLaxMode
			http.SetCookie(w, sid)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/torrents/add":
			sawAdd = true

			err := r.ParseForm()
			if err != nil ||
				!strings.HasPrefix(r.Form.Get("urls"), "magnet:?") {
				t.Errorf("qbit add form = %v, %v", r.Form, err)
			}

			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.QBittorrent.URL = backend.URL
	integrations.QBittorrent.Username = "u"
	integrations.QBittorrent.Password = "p"
	handler := widgetDeps(t, integrations)

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/qbittorrent-add", `{}`),
		http.StatusBadRequest,
		"magnet is required",
	)

	if r := postAction(
		t, handler,
		"/api/actions/qbittorrent-add",
		`{"magnet":"magnet:?xt=urn:btih:abc"}`,
	); r.Code != http.StatusOK {
		t.Fatalf("qbittorrent-add valid = %d, want 200 (%s)", r.Code, r.Body.String())
	}

	if !sawAdd {
		t.Error("qBittorrent backend never received the add request")
	}
	// Non-magnet input passes the handler's empty check but is rejected by the provider.
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/qbittorrent-add", `{"magnet":"not-a-magnet"}`),
		http.StatusBadGateway,
		"add failed",
	)
}

func TestArrAddValidationAndBackend(t *testing.T) {
	rec := new(recorder)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)

		switch {
		case r.URL.Path == "/api/v3/qualityprofile":
			_, _ = w.Write([]byte(`[{"id":7}]`))
		case r.URL.Path == "/api/v3/rootfolder":
			_, _ = w.Write([]byte(`[{"path":"/movies"}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v3/movie/lookup/tmdb"):
			_, _ = w.Write([]byte(`{"title":"Dune","tmdbId":42}`))
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	integrations := new(config.IntegrationsConfig)
	integrations.Radarr.URL = backend.URL
	integrations.Radarr.APIKey = "radarr-key"
	handler := widgetDeps(t, integrations)

	expectActionError(
		t,
		postAction(t, handler, "/api/actions/arr-add", `{`),
		http.StatusBadRequest,
		"invalid request body",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/arr-add", `{}`),
		http.StatusBadRequest,
		"externalId is required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/arr-add", `{"type":"radarr","externalId":0}`),
		http.StatusBadRequest,
		"externalId is required",
	)
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/arr-add", `{"type":"radarr","externalId":-1}`),
		http.StatusBadRequest,
		"externalId is required",
	)

	if r := postAction(
		t, handler,
		"/api/actions/arr-add",
		`{"type":"radarr","title":"Dune","externalId":42}`,
	); r.Code != http.StatusOK {
		t.Fatalf("arr-add valid = %d, want 200 (%s)", r.Code, r.Body.String())
	}

	foundPost := false

	for _, req := range rec.all() {
		if req.Method == http.MethodPost && req.Path == "/api/v3/movie" {
			foundPost = true
		}
	}

	if !foundPost {
		t.Errorf("arr backend never received POST /api/v3/movie; got %+v", rec.all())
	}
	// Unknown arr type reaches the provider, which rejects it.
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/arr-add", `{"type":"bogus","externalId":42}`),
		http.StatusBadGateway,
		"add failed",
	)
}

func TestActionBodySizeLimit(t *testing.T) {
	kube, _ := fakeKube(t, http.StatusOK)
	cfg := config.GetDefaultConfig()
	cfg.Actions.Enabled = true
	deps := actionDeps(cfg)
	deps.Kube = kube
	handler := New(deps).Handler()

	oversized := `{"app":"` + strings.Repeat("a", maxPrefsBytes) + `"}`
	expectActionError(
		t,
		postAction(t, handler, "/api/actions/argo-sync", oversized),
		http.StatusBadRequest,
		"invalid request body",
	)
}
