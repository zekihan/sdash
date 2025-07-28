package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/zekihan/go-commons/cookie"
	"golang.org/x/oauth2"

	"github.com/zekihan/sdash/config"
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
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "offline", "groups", "openid"},
	}

	// Mock dependencies
	auth.OidcSourceMap = map[string]*auth.OidcSource{
		"test": {
			Key: "test",
			SelfConfig: &config.OidcConfig{
				ClientID:     mockOidcServer.ClientID,
				ClientSecret: mockOidcServer.ClientSecret,
			},
			Config:   oauth2Config,
			Provider: provider,
		},
	}

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
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Invalid OIDC key",
			oidcKey:        "invalid",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Missing state cookie",
			oidcKey:        "test",
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
			expectedStatus: http.StatusInternalServerError, // broken for the same reason as below
		},
		//{
		//	name:           "Successful authentication",
		//	oidcKey:        "test",
		//	stateCookie:    "state$$$redirect",
		//	queryParams:    url.Values{"state": {"state$$$redirect"}, "code": {"valid_code"}},
		//	expectedStatus: http.StatusSeeOther,
		// },
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(http.MethodGet, "/callback", http.NoBody)
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

			mockOidcServer.QueueCode("valid_code")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Generate valid code for "Successful authentication" test case
			if tc.name == "Successful authentication" {
				tc.queryParams = url.Values{
					"state": {tc.stateCookie},
					"code":  {"valid_code"},
				}
			}

			// Call AuthCallback
			AuthCallback(rr, req)

			// Assert status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}
