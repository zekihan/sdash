package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	log "github.com/sirupsen/logrus"

	"github.com/zekihan/sdash/config"
)

const (
	defaultOidcInitTimeout = 10 * time.Second
)

var ErrDefaultOidcSourceIsNil = errors.New("defaultOidcSource is nil")

type OidcSource struct {
	SelfConfig *config.OidcConfig
	Config     *oauth2.Config
	Provider   *oidc.Provider
	Key        string
}

func (o *OidcSource) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	oidcConfig := &oidc.Config{
		ClientID: o.SelfConfig.ClientID,
	}
	verifier := o.Provider.Verifier(oidcConfig)

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	return token, nil
}

var (
	Enabled           = false
	OidcSourceMap     = make(map[string]*OidcSource)
	DefaultOidcSource *OidcSource
)

func IsAuthEnabled() bool {
	return Enabled
}

func GetDefaultOidcSource() (*OidcSource, error) {
	if DefaultOidcSource == nil {
		return nil, ErrDefaultOidcSourceIsNil
	}

	return DefaultOidcSource, nil
}

func Init(cfg *config.Config) {
	ctx := context.Background()

	processable := make(map[string]*config.OidcConfig, 0)

	for key, val := range cfg.OidcConfig {
		if val == nil {
			continue
		}

		if !val.Enabled {
			continue
		}

		processable[key] = val
	}

	for key, val := range processable {
		if val.Enabled {
			initSource(ctx, key, val, cfg)
		}
	}

	for key, val := range processable {
		if val.Enabled && val.Default {
			DefaultOidcSource = OidcSourceMap[key]

			break
		}
	}

	if DefaultOidcSource == nil {
		if len(OidcSourceMap) > 0 {
			for key := range OidcSourceMap {
				DefaultOidcSource = OidcSourceMap[key]

				break
			}
		}
	}

	Enabled = len(OidcSourceMap) > 0
}

func initSource(ctx context.Context, key string, val *config.OidcConfig, cfg *config.Config) {
	ctxInner, cancel := context.WithTimeout(ctx, defaultOidcInitTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(ctxInner, val.DiscoveryEndpoint)
	if err != nil {
		cancel()
		//nolint:forbidigo,gocritic // do not want to handle errors here and the defer case already
		// handled above.
		log.Fatalf("Failed to create provider: %v", err)
	}

	redirectURL := fmt.Sprintf("%s/user/oauth2/%s/callback", cfg.WebConfig.BaseURL, key)
	oauth2Config := oauth2.Config{
		ClientID:     val.ClientID,
		ClientSecret: val.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	OidcSourceMap[key] = &OidcSource{
		Key:        key,
		SelfConfig: val,
		Config:     &oauth2Config,
		Provider:   provider,
	}
}
