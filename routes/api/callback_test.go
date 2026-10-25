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

	// Mock dependencies
	oidcConfig := new(config.OidcConfig)
	oidcConfig.ClientID = mockOidcServer.ClientID
	oidcConfig.ClientSecret = mockOidcServer.ClientSecret
	oidcSource := new(auth.OidcSource)
	oidcSource.Key = "test"
	oidcSource.SelfConfig = oidcConfig
	oidcSource.Config = oauth2Config
	oidcSource.Provider = provider
	originalSources := auth.OidcSourceMap
	auth.OidcSourceMap = map[string]*auth.OidcSource{"test": oidcSource}

	cfg := config.GetConfigWithContext(t.Context())
	originalTimeout := cfg.WebConfig.SessionTimeout
	cfg.WebConfig.SessionTimeout = 2 * time.Second

	t.Cleanup(func() { cfg.WebConfig.SessionTimeout = originalTimeout })
	t.Cleanup(func() { auth.OidcSourceMap = originalSources })

	// Test cases
	tests := []struct {
		queryParams    url.Values
		name           string
		oidcKey        string
		stateCookie    string
		expectedStatus int
	}{
		{
			name:           "Missing OIDC key",
			oidcKey:        "",
			queryParams:    nil,
			stateCookie:    "",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Invalid OIDC key",
			oidcKey:        "invalid",
			queryParams:    nil,
			stateCookie:    "",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Missing state cookie",
			oidcKey:        "test",
			queryParams:    nil,
			stateCookie:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "State mismatch",
			oidcKey:        "test",
			stateCookie:    "state1$$$redirect",
			queryParams:    url.Values{"state": {"state2$$$redirect"}},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "State with invalid format",
			oidcKey:        "test",
			stateCookie:    "state1$$$redirect$$$asd",
			queryParams:    url.Values{"state": {"state1$$$redirect$$$asd"}},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "State with empty redirect",
			oidcKey:        "test",
			stateCookie:    "state1$$$",
			queryParams:    url.Values{"state": {"state1$$$"}},
			expectedStatus: http.StatusInternalServerError, // No authorization code was supplied.
		},
		{
			name:        "Provider error containing credentials",
			oidcKey:     "test",
			stateCookie: "state-secret$$$/dashboard",
			queryParams: url.Values{
				"state": {"state-secret$$$/dashboard"},
				"code":  {"code-secret"},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "Successful authentication",
			oidcKey:     "test",
			stateCookie: "state-secret$$$/dashboard",
			queryParams: url.Values{
				"state": {"state-secret$$$/dashboard"},
				"code":  {"code-secret"},
			},
			expectedStatus: http.StatusSeeOther,
		},
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
			req.URL.RawQuery = tc.queryParams.Encode()
			req.SetPathValue("oidc", tc.oidcKey)

			// Set state cookie if provided
			if tc.stateCookie != "" {
				c, err := cookie.CreateNewCookie("state", tc.stateCookie)
				if err != nil {
					t.Fatalf("failed to create cookie: %v", err)
				}

				req.AddCookie(c)
			}

			if tc.name == "Successful authentication" {
				authorizeMockCallback(t, mockOidcServer, oauth2Config, tc.stateCookie)
			}

			if tc.name == "Provider error containing credentials" {
				mockOidcServer.QueueError(&mockoidc.ServerError{
					Code:        http.StatusInternalServerError,
					Error:       mockoidc.InternalServerError,
					Description: "code-secret state-secret",
				})
			}

			rr := loggedCallback(t, req, tc.stateCookie)

			if tc.name == "Successful authentication" {
				assertSessionCookie(t, rr, &cfg.WebConfig)
			}

			// Assert status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}

func authorizeMockCallback(
	t *testing.T,
	server *mockoidc.MockOIDC,
	cfg *oauth2.Config,
	state string,
) {
	t.Helper()
	server.QueueCode("code-secret")

	client := new(http.Client)
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		cfg.AuthCodeURL(state),
		http.NoBody,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}

	closeErr := resp.Body.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorization status: %d", resp.StatusCode)
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

	for _, secret := range []string{"code-secret", "state-secret", state} {
		if secret != "" &&
			(strings.Contains(logs.String(), secret) || strings.Contains(rec.Body.String(), secret)) {
			t.Errorf("callback log leaked credential %q", secret)
		}
	}

	return rec
}
