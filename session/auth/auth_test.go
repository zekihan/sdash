package auth

import (
	"github.com/oauth2-proxy/mockoidc"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zekihan/sdash/config"
)

func TestIsAuthEnabled(t *testing.T) {
	Enabled = false
	assert.False(t, IsAuthEnabled(), "Expected auth to be disabled")

	Enabled = true
	assert.True(t, IsAuthEnabled(), "Expected auth to be enabled")
}

func TestGetDefaultOidcSource(t *testing.T) {
	DefaultOidcSource = nil
	_, err := GetDefaultOidcSource()
	assert.Error(t, err, "Expected error when DefaultOidcSource is nil")

	DefaultOidcSource = &OidcSource{Key: "test"}
	source, err := GetDefaultOidcSource()
	assert.NoError(t, err, "Did not expect an error when DefaultOidcSource is set")
	assert.Equal(t, "test", source.Key, "Expected DefaultOidcSource key to match")
}

func TestInit(t *testing.T) {
	m, err := mockoidc.Run()
	if err != nil {
		t.Fatalf("Failed to start mock OIDC server: %v", err)
	}

	cfg := &config.Config{
		OidcConfig: map[string]*config.OidcConfig{
			"test": {
				Enabled:           true,
				Default:           true,
				ClientID:          m.ClientID,
				ClientSecret:      m.ClientSecret,
				DiscoveryEndpoint: m.Issuer(),
			},
		},
		WebConfig: config.WebConfig{
			BaseURL: "http://localhost:8080",
		},
	}

	Init(cfg)

	assert.True(t, Enabled, "Expected auth to be enabled after Init")
	assert.NotNil(t, DefaultOidcSource, "Expected DefaultOidcSource to be set after Init")
	assert.Equal(t, "test", DefaultOidcSource.Key, "Expected DefaultOidcSource key to match")
	assert.Contains(t, OidcSourceMap, "test", "Expected OidcSourceMap to contain the test key")
}
