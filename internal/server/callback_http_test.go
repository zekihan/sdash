package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/zekihan/go-commons/cookie"
	"golang.org/x/oauth2"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

// setupFakeOIDC starts an in-process OIDC provider (httptest via mockoidc)
// and registers it as the "test" source, restoring globals on cleanup.
func setupFakeOIDC(t *testing.T) *mockoidc.MockOIDC {
	t.Helper()

	mockServer, err := mockoidc.Run()
	if err != nil {
		t.Fatalf("mockoidc.Run: %v", err)
	}

	t.Cleanup(func() { _ = mockServer.Shutdown() })

	provider, err := oidc.NewProvider(t.Context(), mockServer.Issuer())
	if err != nil {
		t.Fatalf("oidc.NewProvider: %v", err)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     mockServer.ClientID,
		ClientSecret: mockServer.ClientSecret,
		RedirectURL:  "/user/oauth2/test/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	oauthCfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	oidcCfg := new(config.OidcConfig)
	oidcCfg.ClientID = mockServer.ClientID
	oidcCfg.ClientSecret = mockServer.ClientSecret

	source := new(auth.OidcSource)
	source.Key = "test"
	source.SelfConfig = oidcCfg
	source.Config = oauthCfg
	source.Provider = provider

	prevMap := auth.OidcSourceMap
	prevDefault := auth.DefaultOidcSource
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": source}
	auth.DefaultOidcSource = source

	t.Cleanup(func() {
		auth.OidcSourceMap = prevMap
		auth.DefaultOidcSource = prevDefault
	})

	cfg := config.GetConfigWithContext(t.Context())
	prevTimeout := cfg.WebConfig.SessionTimeout
	cfg.WebConfig.SessionTimeout = time.Hour

	t.Cleanup(func() { cfg.WebConfig.SessionTimeout = prevTimeout })

	return mockServer
}

func callbackRequest(
	t *testing.T,
	oidcKey, rawQuery, state, verifier string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(),
		http.MethodGet,
		"/user/oauth2/"+oidcKey+"/callback?"+rawQuery,
		http.NoBody,
	)
	req.SetPathValue("oidc", oidcKey)

	for name, value := range map[string]string{stateCookieName: state, verifierCookieName: verifier} {
		if value == "" {
			continue
		}

		c, err := cookie.CreateNewCookie(name, value)
		if err != nil {
			t.Fatalf("CreateNewCookie %s: %v", name, err)
		}

		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	AuthCallback(rec, req)

	return rec
}

const cbVerifier = "test-verifier-0123456789-0123456789-0123456789ab"

func TestCallbackRejectsTamperedAndMissingCookies(t *testing.T) {
	setupFakeOIDC(t)

	state := "state$$$/dashboard"
	query := url.Values{"state": {state}}.Encode()

	// No cookies at all.
	if rec := callbackRequest(t, "test", query, "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("no cookies = %d, want 400", rec.Code)
	}
	// State cookie present but tampered signing must fail to decode.
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/user/oauth2/test/callback?"+query,
		http.NoBody,
	)
	req.SetPathValue("oidc", "test")

	tamperedCookie := new(http.Cookie)
	tamperedCookie.Name = stateCookieName
	tamperedCookie.Value = "tampered-value"
	tamperedCookie.Path = "/"
	tamperedCookie.Secure = true
	tamperedCookie.HttpOnly = true
	tamperedCookie.SameSite = http.SameSiteLaxMode
	req.AddCookie(tamperedCookie)

	rec := httptest.NewRecorder()
	AuthCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("tampered state cookie = %d, want 400", rec.Code)
	}

	// Empty-string state cookie value (present but blank).
	if rec := callbackRequest(t, "test", query, "", cbVerifier); rec.Code != http.StatusBadRequest {
		t.Errorf("missing state with verifier = %d, want 400", rec.Code)
	}

	// State in query missing entirely while cookie exists.
	if rec := callbackRequest(t, "test", "", state, cbVerifier); rec.Code != http.StatusBadRequest {
		t.Errorf("missing query state = %d, want 400 (state mismatch)", rec.Code)
	}

	// Unknown and empty provider keys.
	if rec := callbackRequest(
		t,
		"nope",
		query,
		state,
		cbVerifier,
	); rec.Code != http.StatusNotFound {
		t.Errorf("unknown provider = %d, want 404", rec.Code)
	}

	if rec := callbackRequest(t, "", query, state, cbVerifier); rec.Code != http.StatusNotFound {
		t.Errorf("empty provider = %d, want 404", rec.Code)
	}
}

func TestCallbackProviderErrorAndMissingCode(t *testing.T) {
	mockServer := setupFakeOIDC(t)
	state := "state$$$/dashboard"

	// Provider returned an OAuth error instead of a code: the token exchange
	// cannot succeed, and no session cookie may be issued.
	query := url.Values{
		"state": {state}, "error": {"access_denied"}, "error_description": {"user refused"},
	}.Encode()

	rec := callbackRequest(t, "test", query, state, cbVerifier)
	if rec.Code == http.StatusSeeOther || rec.Code == http.StatusOK {
		t.Fatalf("provider error redirected/OK = %d, must not authenticate", rec.Code)
	}

	assertNoSessionCookie(t, rec)

	// Explicitly queued provider (token endpoint) error.
	mockServer.QueueError(&mockoidc.ServerError{
		Code:        http.StatusBadGateway,
		Error:       mockoidc.InternalServerError,
		Description: "upstream down",
	})

	codeQuery := url.Values{"state": {state}, "code": {"some-code"}}.Encode()

	rec = callbackRequest(t, "test", codeQuery, state, cbVerifier)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("provider token error = %d, want 500", rec.Code)
	}

	assertNoSessionCookie(t, rec)

	// Garbage code with a fresh (unqueued) provider also fails exchange.
	rec = callbackRequest(t, "test", codeQuery, state, cbVerifier)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("invalid code = %d, want 500", rec.Code)
	}

	assertNoSessionCookie(t, rec)
}

func assertNoSessionCookie(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	cfg := config.GetConfigWithContext(t.Context())
	for _, c := range rec.Result().Cookies() {
		if c.Name == cfg.WebConfig.AuthCookie && c.Value != "" && c.MaxAge >= 0 {
			t.Errorf("session cookie issued on failed callback: %+v", c)
		}
	}
}

func TestLoginUnknownProviderAndDefaultWithoutProvider(t *testing.T) {
	prevMap := auth.OidcSourceMap
	prevDefault := auth.DefaultOidcSource

	t.Cleanup(func() {
		auth.OidcSourceMap = prevMap
		auth.DefaultOidcSource = prevDefault
	})

	auth.OidcSourceMap = map[string]*auth.OidcSource{}
	auth.DefaultOidcSource = nil

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/ghost", http.NoBody)
	req.SetPathValue("oidc", "ghost")

	rec := httptest.NewRecorder()
	Login(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("login unknown provider = %d, want 404", rec.Code)
	}

	if len(rec.Result().Cookies()) != 0 {
		t.Error("login for unknown provider set cookies")
	}

	rec = httptest.NewRecorder()
	LoginDefault(
		rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", http.NoBody),
	)

	if rec.Code != http.StatusNotFound {
		t.Errorf("default login with no provider = %d, want 404", rec.Code)
	}
}

func TestFullLoginCallbackRoundTripThroughMux(t *testing.T) {
	mockServer := setupFakeOIDC(t)

	// The callback handler reads the process-wide config, so the mux must be
	// built from that same object for cookie assertions to line up.
	cfg := config.GetConfigWithContext(t.Context())
	deps := new(Deps)
	deps.Config = cfg
	deps.StartedAt = time.Now()
	handler := New(deps).Handler()

	loginRec, state := startLoginViaMux(t, handler, mockServer)
	callbackReq, callbackRec := finishCallbackViaMux(t, handler, mockServer, loginRec, state, cfg)

	assertSessionCookie(t, callbackRec, &cfg.WebConfig)
	assertVerifierCleared(t, callbackRec)
	assertCallbackSessionAuthenticates(t, handler, callbackRec, cfg)
	assertCallbackReplayRejected(t, handler, loginRec, callbackReq)
}

func startLoginViaMux(
	t *testing.T,
	handler http.Handler,
	mockServer *mockoidc.MockOIDC,
) (*httptest.ResponseRecorder, string) {
	t.Helper()

	loginReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/login/test?redirect=/settings",
		http.NoBody,
	)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusSeeOther {
		t.Fatalf("login via mux = %d, want 303 (%s)", loginRec.Code, loginRec.Body.String())
	}

	authURL, err := url.Parse(loginRec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(authURL.String(), mockServer.Issuer()) {
		t.Fatalf("login Location = %q, want fake provider %q", authURL, mockServer.Issuer())
	}

	state := authURL.Query().Get("state")
	if state == "" || !strings.HasSuffix(state, "$$$/settings") {
		t.Fatalf("state = %q, want suffix $$$/settings", state)
	}

	return loginRec, state
}

func finishCallbackViaMux(
	t *testing.T,
	handler http.Handler,
	mockServer *mockoidc.MockOIDC,
	loginRec *httptest.ResponseRecorder,
	state string,
	cfg *config.Config,
) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	// Carry the state/verifier cookies into the callback, as a browser would.
	callbackReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/user/oauth2/test/callback?"+url.Values{"state": {state}}.Encode(), http.NoBody)

	verifierForChallenge := ""

	for _, loginCookie := range loginRec.Result().Cookies() {
		callbackReq.AddCookie(loginCookie)

		if loginCookie.Name != verifierCookieName {
			continue
		}

		// Recover the plaintext verifier to bind the mock code via PKCE.
		var verifierValue string

		probe := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		probe.AddCookie(loginCookie)

		err := cookie.GetCookie(probe, verifierCookieName, &verifierValue)
		if err != nil {
			t.Fatal(err)
		}

		verifierForChallenge = verifierValue
	}

	_ = cfg
	query := callbackReq.URL.Query()
	setMockCallbackCode(t, mockServer, query, verifierForChallenge)
	callbackReq.URL.RawQuery = query.Encode()

	callbackRec := httptest.NewRecorder()
	handler.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusSeeOther {
		t.Fatalf(
			"callback via mux = %d, want 303 (%s)",
			callbackRec.Code,
			callbackRec.Body.String(),
		)
	}

	if loc := callbackRec.Header().Get("Location"); loc != "/settings" {
		t.Errorf("callback Location = %q, want /settings", loc)
	}

	return callbackReq, callbackRec
}

func assertCallbackSessionAuthenticates(
	t *testing.T,
	handler http.Handler,
	callbackRec *httptest.ResponseRecorder,
	cfg *config.Config,
) {
	t.Helper()

	prevEnabled := auth.Enabled
	auth.Enabled = true

	t.Cleanup(func() { auth.Enabled = prevEnabled })

	var sessionCookie *http.Cookie

	for _, issuedCookie := range callbackRec.Result().Cookies() {
		if issuedCookie.Name == cfg.WebConfig.AuthCookie {
			sessionCookie = issuedCookie
		}
	}

	if sessionCookie == nil {
		t.Fatal("callback issued no session cookie")
	}

	protectedReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/api/health/services",
		http.NoBody,
	)
	protectedReq.AddCookie(sessionCookie)

	protectedRec := httptest.NewRecorder()
	handler.ServeHTTP(protectedRec, protectedReq)

	if protectedRec.Code != http.StatusOK {
		t.Errorf("protected with callback session = %d, want 200", protectedRec.Code)
	}
}

func assertCallbackReplayRejected(
	t *testing.T,
	handler http.Handler,
	loginRec *httptest.ResponseRecorder,
	callbackReq *http.Request,
) {
	t.Helper()

	replay := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		callbackReq.URL.String(),
		http.NoBody,
	)
	for _, staleCookie := range loginRec.Result().Cookies() {
		replay.AddCookie(staleCookie) // stale verifier cookie replayed by an attacker
	}

	replayRec := httptest.NewRecorder()
	handler.ServeHTTP(replayRec, replay)

	if replayRec.Code == http.StatusSeeOther {
		t.Error("replayed callback redirected as if freshly authenticated")
	}
}
