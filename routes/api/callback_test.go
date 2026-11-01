package api

import (
	"bytes"
	"log/slog"
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
	"github.com/zekihan/sdash/requestlog"
	"github.com/zekihan/sdash/session/auth"
)

func TestAuthCallback(t *testing.T) {
	mockOidcServer, err := mockoidc.Run()
	if err != nil {
		t.Fatalf("Failed to start mock OIDC server: %v", err)
	}

	t.Cleanup(func() {
		shutdownErr := mockOidcServer.Shutdown()
		if shutdownErr != nil {
			t.Errorf("Failed to shutdown mock OIDC server: %v", shutdownErr)
		}
	})

	provider, err := oidc.NewProvider(t.Context(), mockOidcServer.Issuer())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     mockOidcServer.ClientID,
		ClientSecret: mockOidcServer.ClientSecret,
		RedirectURL:  "/user/oauth2/test/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	oauth2Config.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	originalSources := auth.OidcSourceMap

	t.Cleanup(func() { auth.OidcSourceMap = originalSources })

	// Mock dependencies
	oidcConfig := new(config.OidcConfig)
	oidcConfig.ClientID = mockOidcServer.ClientID
	oidcConfig.ClientSecret = mockOidcServer.ClientSecret
	oidcSource := new(auth.OidcSource)
	oidcSource.Key = "test"
	oidcSource.SelfConfig = oidcConfig
	oidcSource.Config = oauth2Config
	oidcSource.Provider = provider
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": oidcSource}

	cfg := config.GetConfigWithContext(t.Context())
	originalTimeout := cfg.WebConfig.SessionTimeout
	cfg.WebConfig.SessionTimeout = 2 * time.Second

	t.Cleanup(func() { cfg.WebConfig.SessionTimeout = originalTimeout })

	// Test cases
	type callbackTestCase struct {
		queryParams      url.Values
		name             string
		oidcKey          string
		stateCookie      string
		verifierCookie   string
		expectedLocation string
		expectedStatus   int
	}

	const wrongVerifierCase = "Wrong verifier is rejected by the token endpoint"

	const testVerifier = "test-verifier-0123456789-0123456789-0123456789ab"

	errorCases := []callbackTestCase{
		{
			name:             "Missing OIDC key",
			oidcKey:          "",
			queryParams:      nil,
			stateCookie:      "",
			verifierCookie:   "",
			expectedStatus:   http.StatusNotFound,
			expectedLocation: "",
		},
		{
			name:             "Invalid OIDC key",
			oidcKey:          "invalid",
			queryParams:      nil,
			stateCookie:      "",
			verifierCookie:   "",
			expectedStatus:   http.StatusNotFound,
			expectedLocation: "",
		},
		{
			name:             "Missing state cookie",
			oidcKey:          "test",
			queryParams:      nil,
			stateCookie:      "",
			verifierCookie:   "",
			expectedStatus:   http.StatusBadRequest,
			expectedLocation: "",
		},
		{
			name:             "Missing verifier cookie",
			oidcKey:          "test",
			stateCookie:      "state$$$/dashboard",
			verifierCookie:   "",
			queryParams:      url.Values{"state": {"state$$$/dashboard"}},
			expectedStatus:   http.StatusBadRequest,
			expectedLocation: "",
		},
		{
			name:             wrongVerifierCase,
			oidcKey:          "test",
			stateCookie:      "state$$$/dashboard",
			verifierCookie:   "a-different-verifier-0123456789-0123456789-abcdef",
			queryParams:      url.Values{"state": {"state$$$/dashboard"}},
			expectedStatus:   http.StatusInternalServerError,
			expectedLocation: "",
		},
		{
			name:             "State mismatch",
			oidcKey:          "test",
			stateCookie:      "state1$$$redirect",
			verifierCookie:   "",
			queryParams:      url.Values{"state": {"state2$$$redirect"}},
			expectedStatus:   http.StatusBadRequest,
			expectedLocation: "",
		},
		{
			name:             "State with invalid format",
			oidcKey:          "test",
			stateCookie:      "state-without-separator",
			verifierCookie:   "",
			queryParams:      url.Values{"state": {"state-without-separator"}},
			expectedStatus:   http.StatusBadRequest,
			expectedLocation: "",
		},
		{
			name:             "Token exchange without a code fails",
			oidcKey:          "test",
			stateCookie:      "state1$$$redirect$$$asd",
			verifierCookie:   testVerifier,
			queryParams:      url.Values{"state": {"state1$$$redirect$$$asd"}},
			expectedStatus:   http.StatusInternalServerError,
			expectedLocation: "",
		},
		{
			name:             "State with empty redirect",
			oidcKey:          "test",
			stateCookie:      "state1$$$",
			verifierCookie:   testVerifier,
			queryParams:      url.Values{"state": {"state1$$$"}},
			expectedStatus:   http.StatusInternalServerError,
			expectedLocation: "",
		},
		{
			name:           "Provider error containing credentials",
			oidcKey:        "test",
			stateCookie:    "state-secret$$$/dashboard",
			verifierCookie: testVerifier,
			queryParams: url.Values{
				"state": {"state-secret$$$/dashboard"},
				"code":  {"code-secret"},
			},
			expectedStatus:   http.StatusInternalServerError,
			expectedLocation: "",
		},
	}

	redirectCases := redirectTestCases()
	tests := make([]callbackTestCase, 0, len(errorCases)+len(redirectCases))
	tests = append(tests, errorCases...)

	for _, redirect := range redirectCases {
		state := "state$$$" + redirect.target
		tests = append(tests, callbackTestCase{
			queryParams:      url.Values{"state": {state}},
			name:             "Successful authentication/" + redirect.target,
			oidcKey:          "test",
			stateCookie:      state,
			verifierCookie:   testVerifier,
			expectedStatus:   http.StatusSeeOther,
			expectedLocation: redirect.want,
		})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/callback",
				http.NoBody,
			)

			if tc.expectedStatus == http.StatusSeeOther || tc.name == wrongVerifierCase {
				setMockCallbackCode(t, mockOidcServer, tc.queryParams, testVerifier)
			}

			req.URL.RawQuery = tc.queryParams.Encode()
			req.SetPathValue("oidc", tc.oidcKey)

			addCallbackCookies(t, req, tc.stateCookie, tc.verifierCookie)

			if tc.name == "Provider error containing credentials" {
				mockOidcServer.QueueError(&mockoidc.ServerError{
					Code:        http.StatusInternalServerError,
					Error:       mockoidc.InternalServerError,
					Description: "code-secret state-secret",
				})
			}

			rr := loggedCallback(t, req, tc.stateCookie)

			// Assert status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != tc.expectedLocation {
				t.Errorf("Location = %q, want %q", got, tc.expectedLocation)
			}

			if tc.expectedStatus == http.StatusSeeOther {
				assertSessionCookie(t, rr, &cfg.WebConfig)
				assertVerifierCleared(t, rr)
			}
		})
	}
}

func loggedCallback(t *testing.T, req *http.Request, state string) *httptest.ResponseRecorder {
	t.Helper()

	var logs bytes.Buffer

	originalLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(originalLogger)

	rec := httptest.NewRecorder()
	requestlog.Logging(http.HandlerFunc(AuthCallback)).ServeHTTP(rec, req)

	for _, secret := range []string{"code-secret", "state-secret", state, req.URL.Query().Get("code")} {
		if secret != "" &&
			(strings.Contains(logs.String(), secret) || strings.Contains(rec.Body.String(), secret)) {
			t.Errorf("callback log leaked credential %q", secret)
		}
	}

	return rec
}

func addCallbackCookies(t *testing.T, req *http.Request, state, verifier string) {
	t.Helper()

	for name, value := range map[string]string{
		stateCookieName:    state,
		verifierCookieName: verifier,
	} {
		if value == "" {
			continue
		}

		c, err := cookie.CreateNewCookie(name, value)
		if err != nil {
			t.Fatalf("failed to create cookie: %v", err)
		}

		req.AddCookie(c)
	}
}

func assertVerifierCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	response := rec.Result()
	defer response.Body.Close()

	for _, c := range response.Cookies() {
		if c.Name != verifierCookieName {
			continue
		}

		if c.MaxAge >= 0 || c.Value != "" || c.Path != "/" || !c.Secure || !c.HttpOnly {
			t.Errorf("verifier cookie not cleared properly: %+v", c)
		}

		return
	}

	t.Error("verifier cookie was not cleared")
}

// setMockCallbackCode registers an authorization code bound to the S256
// challenge of verifier, so the mock token endpoint enforces PKCE.
func setMockCallbackCode(
	t *testing.T,
	server *mockoidc.MockOIDC,
	query url.Values,
	verifier string,
) {
	t.Helper()

	challenge, err := mockoidc.GenerateCodeChallenge(mockoidc.CodeChallengeMethodS256, verifier)
	if err != nil {
		t.Fatalf("failed to compute code challenge: %v", err)
	}

	mockSession, err := server.SessionStore.NewSession(
		"openid profile email", "", mockoidc.DefaultUser(),
		challenge, mockoidc.CodeChallengeMethodS256,
	)
	if err != nil {
		t.Fatalf("failed to create mock session: %v", err)
	}

	query.Set("code", mockSession.SessionID)
}
