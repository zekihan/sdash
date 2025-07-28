package auth

import (
	"testing"

	"github.com/oauth2-proxy/mockoidc"

	"github.com/zekihan/sdash/config"
)

func TestIsAuthEnabled(t *testing.T) {
	Enabled = false

	if IsAuthEnabled() {
		t.Errorf("Expected auth to be disabled")
	}

	Enabled = true

	if !IsAuthEnabled() {
		t.Errorf("Expected auth to be enabled")
	}
}

func TestGetDefaultOidcSource(t *testing.T) {
	DefaultOidcSource = nil

	_, err := GetDefaultOidcSource()
	if err == nil {
		t.Errorf("Expected error when DefaultOidcSource is nil")
	}

	DefaultOidcSource = &OidcSource{Key: "test"}

	source, err := GetDefaultOidcSource()
	if err != nil {
		t.Errorf("Did not expect an error when DefaultOidcSource is set: %v", err)
	}

	if source.Key != "test" {
		t.Errorf("Expected DefaultOidcSource key to match, got %v", source.Key)
	}
}

func TestInit(t *testing.T) {
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

	cfg := &config.Config{
		OidcConfig: map[string]*config.OidcConfig{
			"test": {
				Enabled:           true,
				Default:           true,
				ClientID:          mockOidcServer.ClientID,
				ClientSecret:      mockOidcServer.ClientSecret,
				DiscoveryEndpoint: mockOidcServer.Issuer(),
			},
		},
		WebConfig: config.WebConfig{
			BaseURL: "http://localhost:8080",
		},
	}

	Init(cfg)

	if !Enabled {
		t.Errorf("Expected auth to be enabled after Init")
	}

	if DefaultOidcSource == nil {
		t.Errorf("Expected DefaultOidcSource to be set after Init")
	}

	if DefaultOidcSource != nil && DefaultOidcSource.Key != "test" {
		t.Errorf("Expected DefaultOidcSource key to match, got %v", DefaultOidcSource.Key)
	}

	if _, ok := OidcSourceMap["test"]; !ok {
		t.Errorf("Expected OidcSourceMap to contain the test key")
	}
}
