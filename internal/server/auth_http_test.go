package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/session"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

// saveAuthState snapshots the process-wide auth globals mutated by tests in
// this package and restores them on cleanup. Tests using it must not run in
// parallel.
func saveAuthState(t *testing.T) {
	t.Helper()

	prevEnabled := auth.Enabled
	prevMap := auth.OidcSourceMap
	prevDefault := auth.DefaultOidcSource

	t.Cleanup(func() {
		auth.Enabled = prevEnabled
		auth.OidcSourceMap = prevMap
		auth.DefaultOidcSource = prevDefault
	})
}

func authTestServer(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()

	deps := new(Deps)
	deps.Config = cfg
	deps.StartedAt = time.Now()

	return New(deps).Handler()
}

func mintSessionCookie(t *testing.T, cfg *config.Config) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()

	err := setAuthenticatedSession(rec, &cfg.WebConfig)
	if err != nil {
		t.Fatalf("setAuthenticatedSession: %v", err)
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == cfg.WebConfig.AuthCookie {
			return c
		}
	}

	t.Fatal("no session cookie issued")

	return nil
}

func doReq(
	t *testing.T,
	handler http.Handler,
	method, target string,
	sessionCookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestPublicRoutesOpenAndProtectedClosedWhenAuthEnabled(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = true
	auth.OidcSourceMap = map[string]*auth.OidcSource{}

	cfg := config.GetDefaultConfig()
	handler := authTestServer(t, cfg)

	for _, tc := range []struct {
		path string
		body string
		want int
	}{
		{path: "/api/ping", want: http.StatusOK, body: "pong"},
		{path: "/api/health", want: http.StatusOK, body: "ok"},
	} {
		rec := doReq(t, handler, http.MethodGet, tc.path, nil)
		if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf(
				"GET %s unauthenticated = %d %q, want %d containing %q",
				tc.path,
				rec.Code,
				rec.Body.String(),
				tc.want,
				tc.body,
			)
		}

		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("GET %s Content-Type = %q, want application/json", tc.path, ct)
		}
	}

	protectedGET := []string{
		"/api/dashboard", "/api/health/services", "/api/system", "/api/docker",
		"/api/weather", "/api/kubernetes", "/api/proxmox", "/api/pbs",
		"/api/truenas", "/api/crowdsec", "/api/preferences", "/api/calendar",
		"/api/media-rail", "/api/gatus", "/api/service-widgets", "/api/events",
		"/api/proxy/icon?url=https://example.com/icon.png",
		"/api/arr/lookup?type=sonarr&term=x",
	}
	for _, path := range protectedGET {
		rec := doReq(t, handler, http.MethodGet, path, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf(
				"GET %s unauthenticated = %d, want 401 (body %q)",
				path,
				rec.Code,
				rec.Body.String(),
			)

			continue
		}

		var body map[string]string

		err := json.Unmarshal(
			rec.Body.Bytes(),
			&body,
		)
		if err != nil ||
			body["error"] != "unauthorized" {
			t.Errorf("GET %s body = %q, want {\"error\":\"unauthorized\"}", path, rec.Body.String())
		}
	}

	for _, path := range []string{
		"/api/actions/argo-sync", "/api/actions/restart", "/api/actions/proxmox-start",
		"/api/actions/wol", "/api/actions/homeassistant",
		"/api/actions/qbittorrent-add", "/api/actions/arr-add",
	} {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			path,
			strings.NewReader(`{}`),
		)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s unauthenticated = %d, want 401", path, rec.Code)
		}
	}

	rec := doReq(t, handler, http.MethodPost, "/api/logout", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/logout unauthenticated = %d, want 401", rec.Code)
	}
}

func TestProtectedRoutesAcceptValidSessionAndRejectBadSessions(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = true
	auth.OidcSourceMap = map[string]*auth.OidcSource{}

	cfg := config.GetDefaultConfig()
	cfg.WebConfig.SessionTimeout = time.Hour
	handler := authTestServer(t, cfg)

	valid := mintSessionCookie(t, cfg)
	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		valid,
	); rec.Code != http.StatusOK {
		t.Errorf(
			"valid session GET /api/health/services = %d, want 200 (%s)",
			rec.Code,
			rec.Body.String(),
		)
	}

	// Expired session embedded in a correctly signed cookie.
	expiredSess := new(session.Cookie)
	expiredSess.Created = time.Now().Add(-2 * time.Hour)
	expiredSess.ExpireTime = time.Now().Add(-time.Hour)
	expiredSess.Authenticated = true

	expiredCookie, err := cookie.CreateNewCookie(cfg.WebConfig.AuthCookie, *expiredSess)
	if err != nil {
		t.Fatal(err)
	}

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		expiredCookie,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("expired session = %d, want 401", rec.Code)
	}

	// Signed but never authenticated.
	anonSess := new(session.Cookie)
	anonSess.Created = time.Now()
	anonSess.ExpireTime = time.Now().Add(time.Hour)
	anonSess.Authenticated = false

	anonCookie, err := cookie.CreateNewCookie(cfg.WebConfig.AuthCookie, *anonSess)
	if err != nil {
		t.Fatal(err)
	}

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		anonCookie,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated session cookie = %d, want 401", rec.Code)
	}

	// Tampered value fails securecookie decoding. The copy inherits the
	// issued cookie's Secure/HttpOnly/SameSite attributes.
	tampered := *valid //nolint:gosec // test fixture copies a hardened issued cookie
	if len(tampered.Value) > 4 {
		tampered.Value = tampered.Value[:len(tampered.Value)-4] + "AAAA"
	}

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		&tampered,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("tampered session = %d, want 401", rec.Code)
	}

	// Garbage and wrong-name cookies.
	garbage := new(http.Cookie)
	garbage.Name = cfg.WebConfig.AuthCookie
	garbage.Value = "not-a-session"
	garbage.Path = "/"
	garbage.Secure = true
	garbage.HttpOnly = true
	garbage.SameSite = http.SameSiteLaxMode

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		garbage,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage session = %d, want 401", rec.Code)
	}

	wrongName := new(http.Cookie)
	wrongName.Name = "OTHER"
	wrongName.Value = valid.Value
	wrongName.Path = "/"
	wrongName.Secure = true
	wrongName.HttpOnly = true
	wrongName.SameSite = http.SameSiteLaxMode

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		wrongName,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong-name session = %d, want 401", rec.Code)
	}
}

func TestProtectedRoutesOpenWhenAuthDisabled(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = false
	auth.OidcSourceMap = map[string]*auth.OidcSource{}

	cfg := config.GetDefaultConfig()
	handler := authTestServer(t, cfg)

	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		nil,
	); rec.Code != http.StatusOK {
		t.Errorf("auth disabled GET /api/health/services = %d, want 200", rec.Code)
	}

	if rec := doReq(t, handler, http.MethodGet, "/api/ping", nil); rec.Code != http.StatusOK {
		t.Errorf("auth disabled GET /api/ping = %d, want 200", rec.Code)
	}
}

func TestHandleMeReportsAuthStateAndProviders(t *testing.T) {
	saveAuthState(t)

	cfg := config.GetDefaultConfig()
	cfg.WebConfig.SessionTimeout = time.Hour
	handler := authTestServer(t, cfg)

	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()

		var body map[string]any

		err := json.Unmarshal(rec.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("decode /api/me %q: %v", rec.Body.String(), err)
		}

		return body
	}

	// Auth disabled: SPA is told it is already authenticated, no providers.
	auth.Enabled = false
	auth.OidcSourceMap = map[string]*auth.OidcSource{}

	body := decode(doReq(t, handler, http.MethodGet, "/api/me", nil))
	if body["authEnabled"] != false || body["authenticated"] != true {
		t.Errorf("auth disabled /api/me = %v, want authEnabled=false authenticated=true", body)
	}

	// Auth enabled, no cookie.
	auth.Enabled = true
	srcB := testOidcSource("beta")
	srcA := testOidcSource("alpha")
	auth.OidcSourceMap = map[string]*auth.OidcSource{"beta": srcB, "alpha": srcA}
	auth.DefaultOidcSource = srcB

	body = decode(doReq(t, handler, http.MethodGet, "/api/me", nil))
	if body["authEnabled"] != true || body["authenticated"] != false {
		t.Errorf(
			"auth enabled anonymous /api/me = %v, want authEnabled=true authenticated=false",
			body,
		)
	}

	providers, ok := body["providers"].([]any)
	if !ok || len(providers) != 2 {
		t.Fatalf("providers = %v, want 2 entries", body["providers"])
	}

	first, _ := providers[0].(map[string]any)

	second, _ := providers[1].(map[string]any)
	if first["key"] != "alpha" || first["default"] != false || second["key"] != "beta" ||
		second["default"] != true {
		t.Errorf("providers not sorted/default-flagged: %v", providers)
	}

	// Auth enabled with a valid session.
	valid := mintSessionCookie(t, cfg)

	body = decode(doReq(t, handler, http.MethodGet, "/api/me", valid))
	if body["authenticated"] != true {
		t.Errorf("authenticated /api/me = %v, want authenticated=true", body)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	saveAuthState(t)

	auth.Enabled = true
	auth.OidcSourceMap = map[string]*auth.OidcSource{}

	cfg := config.GetDefaultConfig()
	cfg.WebConfig.SessionTimeout = time.Hour
	handler := authTestServer(t, cfg)
	valid := mintSessionCookie(t, cfg)

	rec := doReq(t, handler, http.MethodPost, "/api/logout", valid)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var body map[string]bool

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil || !body["ok"] {
		t.Errorf("logout body = %q, want ok=true", rec.Body.String())
	}

	found := false

	for _, c := range rec.Result().Cookies() {
		if c.Name == cfg.WebConfig.AuthCookie {
			found = true

			if c.Value != "" || c.MaxAge >= 0 || c.Path != "/" {
				t.Errorf("logout did not clear cookie: %+v", c)
			}
		}
	}

	if !found {
		t.Error("logout set no clearing cookie")
	}

	// After logout the browser holds no valid cookie, so protected routes 401.
	if rec := doReq(
		t, handler,
		http.MethodGet,
		"/api/health/services",
		nil,
	); rec.Code != http.StatusUnauthorized {
		t.Errorf("post-logout protected = %d, want 401", rec.Code)
	}
}

func TestSessionCookieIssuanceAttributes(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.WebConfig.SessionTimeout = 90 * time.Minute

	rec := httptest.NewRecorder()
	before := time.Now()

	err := setAuthenticatedSession(rec, &cfg.WebConfig)
	if err != nil {
		t.Fatal(err)
	}

	c := mintSessionCookie(t, cfg)
	_ = c

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("issued %d cookies, want 1", len(cookies))
	}

	issued := cookies[0]
	if issued.MaxAge != int((90*time.Minute)/time.Second) {
		t.Errorf("MaxAge = %d, want %d", issued.MaxAge, int((90*time.Minute)/time.Second))
	}

	if issued.Expires.Before(before.Add(89*time.Minute)) ||
		issued.Expires.After(time.Now().Add(91*time.Minute)) {
		t.Errorf("Expires = %v, want ~90m after issuance", issued.Expires)
	}

	if !issued.Secure || !issued.HttpOnly || issued.SameSite != http.SameSiteLaxMode ||
		issued.Path != "/" {
		t.Errorf("cookie flags = %+v, want Secure/HttpOnly/SameSite=Lax/Path=/", issued)
	}

	var sess session.Cookie

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.AddCookie(issued)

	err = cookie.GetCookie(req, cfg.WebConfig.AuthCookie, &sess)
	if err != nil {
		t.Fatal(err)
	}

	if !sess.Authenticated || sess.IsExpired() {
		t.Errorf("issued session = %+v, want authenticated and unexpired", sess)
	}

	if delta := sess.ExpireTime.Sub(sess.Created); delta != cfg.WebConfig.SessionTimeout {
		t.Errorf("embedded lifetime = %v, want %v", delta, cfg.WebConfig.SessionTimeout)
	}
}
