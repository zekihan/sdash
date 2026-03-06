package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

func TestLogin(t *testing.T) {
	// Mock OIDC source
	auth.OidcSourceMap = map[string]*auth.OidcSource{
		"test": {
			Key:    "test",
			Config: &oauth2.Config{},
		},
	}

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
	auth.OidcSourceMap = map[string]*auth.OidcSource{
		"test": {
			Key:    "test",
			Config: &oauth2.Config{},
		},
	}

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

	auth.DefaultOidcSource = &auth.OidcSource{
		Key:    "test",
		Config: &oauth2.Config{},
	}

	t.Run("Redirect to Default OIDC", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", http.NoBody)
		rec := httptest.NewRecorder()

		LoginDefault(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
		}
	})
}
