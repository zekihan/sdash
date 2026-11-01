package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zekihan/go-commons/cookie"
	"golang.org/x/oauth2"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

func TestLogin(t *testing.T) {
	// Mock OIDC source
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": testOidcSource("test")}

	cfg := config.GetConfigWithContext(t.Context())
	cfg.OidcConfig = map[string]*config.OidcConfig{
		"test": {
			Enabled:           true,
			Default:           true,
			ClientID:          "test-client-id",
			ClientSecret:      "test-client-secret",
			DiscoveryEndpoint: "",
		},
	}

	t.Run("Valid OIDC Key", func(t *testing.T) {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/login/test",
			http.NoBody,
		)
		rec := httptest.NewRecorder()

		req.SetPathValue("oidc", "test")
		Login(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
		}

		assertPKCE(t, rec)
	})

	t.Run("Invalid OIDC Key", func(t *testing.T) {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/login/invalid",
			http.NoBody,
		)
		rec := httptest.NewRecorder()

		req.SetPathValue("oidc", "invalid")
		Login(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestLoginDefault(t *testing.T) {
	// Mock OIDC source
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": testOidcSource("test")}

	cfg := config.GetConfigWithContext(t.Context())
	cfg.OidcConfig = map[string]*config.OidcConfig{
		"test": {
			Enabled:           true,
			Default:           true,
			ClientID:          "test-client-id",
			ClientSecret:      "test-client-secret",
			DiscoveryEndpoint: "",
		},
	}

	auth.DefaultOidcSource = testOidcSource("test")

	t.Run("Redirect to Default OIDC", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", http.NoBody)
		rec := httptest.NewRecorder()

		LoginDefault(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
		}
	})
}

func testOidcSource(key string) *auth.OidcSource {
	source := new(auth.OidcSource)
	source.Key = key
	source.Config = new(oauth2.Config)

	return source
}

func TestLoginReturnPaths(t *testing.T) {
	originalSources, originalDefault := auth.OidcSourceMap, auth.DefaultOidcSource

	t.Cleanup(func() {
		auth.OidcSourceMap, auth.DefaultOidcSource = originalSources, originalDefault
	})

	source := testOidcSource("test")
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": source}
	auth.DefaultOidcSource = source

	for _, tc := range redirectTestCases() {
		t.Run(tc.target, func(t *testing.T) {
			testDefaultLoginReturnPath(t, tc.target, tc.want)
			testProviderLoginReturnPath(t, tc.target, tc.want)
		})
	}
}

func testDefaultLoginReturnPath(t *testing.T, target, want string) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/login?"+url.Values{"redirect": {target}}.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	LoginDefault(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("default login status = %d", rec.Code)
	}

	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}

	if location.IsAbs() || location.Host != "" || location.Path != "/login/test" {
		t.Fatalf("unsafe default login Location: %q", location)
	}

	if got := location.Query().Get("redirect"); got != want {
		t.Errorf("default login return URL = %q, want %q", got, want)
	}
}

func testProviderLoginReturnPath(t *testing.T, target, want string) {
	t.Helper()

	// Exercise direct provider login too, without relying on LoginDefault's validation.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/login/test?"+url.Values{"redirect": {target}}.Encode(), http.NoBody)
	req.SetPathValue("oidc", "test")

	rec := httptest.NewRecorder()
	Login(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("provider login status = %d", rec.Code)
	}

	authURL, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}

	state := authURL.Query().Get("state")

	parts := strings.SplitN(state, "$$$", 2)
	if len(parts) != 2 || parts[1] != want {
		t.Fatalf("unexpected return URL in OAuth state: %q", state)
	}

	response := rec.Result()
	defer response.Body.Close()

	cookieRequest := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/",
		http.NoBody,
	)
	for _, stateCookie := range response.Cookies() {
		cookieRequest.AddCookie(stateCookie)
	}

	var signedState string

	err = cookie.GetCookie(cookieRequest, "state", &signedState)
	if err != nil {
		t.Fatal(err)
	}

	if signedState != state {
		t.Error("OAuth state did not match signed cookie")
	}
}

// assertPKCE checks the auth URL carries an S256 challenge and that the
// verifier cookie holds the matching verifier with the same flags as state.
func assertPKCE(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	authURL, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}

	if got := authURL.Query().Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}

	challenge := authURL.Query().Get("code_challenge")
	if challenge == "" {
		t.Fatal("code_challenge missing from auth URL")
	}

	response := rec.Result()
	defer response.Body.Close()

	verifierCookie := findCookie(response, verifierCookieName)
	stateCookie := findCookie(response, stateCookieName)

	if verifierCookie == nil || stateCookie == nil {
		t.Fatalf("missing cookies: verifier=%v state=%v", verifierCookie, stateCookie)
	}

	assertSameFlags(t, verifierCookie, stateCookie)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.AddCookie(verifierCookie)

	var verifier string

	err = cookie.GetCookie(req, verifierCookieName, &verifier)
	if err != nil {
		t.Fatal(err)
	}

	if want := oauth2.S256ChallengeFromVerifier(verifier); challenge != want {
		t.Errorf("code_challenge %q does not match verifier cookie (want %q)", challenge, want)
	}

	if strings.Contains(authURL.RawQuery, verifier) {
		t.Error("verifier leaked into auth URL")
	}
}

func findCookie(response *http.Response, name string) *http.Cookie {
	for _, candidate := range response.Cookies() {
		if candidate.Name == name {
			return candidate
		}
	}

	return nil
}

func assertSameFlags(t *testing.T, got, want *http.Cookie) {
	t.Helper()

	if !got.Secure || !got.HttpOnly || got.SameSite != http.SameSiteLaxMode ||
		got.Path != want.Path || got.MaxAge != want.MaxAge ||
		got.SameSite != want.SameSite {
		t.Errorf("verifier cookie flags differ from state cookie: %+v vs %+v", got, want)
	}
}
