package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/sethvargo/go-envconfig/v2"
	"gopkg.in/yaml.v3"
)

const (
	EnvKeyPrefix              = "SDS__"
	defaultConfigFile         = "config.yaml"
	defaultPort               = 8080
	defaultSessionTimeout     = 24 * time.Hour
	maxSessionTimeout         = 30 * 24 * time.Hour
	defaultPollInterval       = 5 * time.Second
	defaultCookieHashKeySize  = 64
	defaultCookieBlockKeySize = 32
	defaultHealthInterval     = 30 * time.Second
	defaultHealthTimeout      = 5 * time.Second
	defaultHealthHistory      = 90
	maxHealthHistory          = 1000
)

var config *Config

var (
	ErrInvalidSessionTimeout = errors.New(
		"sessionTimeout must be a whole number of seconds between 1s and 720h",
	)
	ErrInvalidConfigDocument = errors.New(
		"config must contain exactly one YAML mapping (use {} for defaults)",
	)
)

type Config struct {
	OidcConfig   map[string]*OidcConfig `yaml:"oidc"`
	Integrations IntegrationsConfig     `yaml:"integrations" env:", prefix=INTEGRATIONS_"`
	Kubernetes   KubernetesConfig       `yaml:"kubernetes"   env:", prefix=KUBERNETES_"`
	Proxmox      ProxmoxConfig          `yaml:"proxmox"      env:", prefix=PROXMOX_"`
	PBS          PBSConfig              `yaml:"pbs"          env:", prefix=PBS_"`
	TrueNAS      TrueNASConfig          `yaml:"truenas"      env:", prefix=TRUENAS_"`
	CrowdSec     CrowdSecConfig         `yaml:"crowdsec"     env:", prefix=CROWDSEC_"`
	Actions      ActionsConfig          `yaml:"actions"      env:", prefix=ACTIONS_"`
	Preferences  PreferencesConfig      `yaml:"preferences"  env:", prefix=PREFERENCES_"`
	Docker       DockerConfig           `yaml:"docker"       env:", prefix=DOCKER_"`
	Gatus        GatusConfig            `yaml:"gatus"        env:", prefix=GATUS_"`
	WebConfig    WebConfig              `yaml:"web"          env:", prefix=WEB_"`
	LogConfig    LogConfig              `yaml:"log"          env:", prefix=LOG_"`
	Weather      WeatherConfig          `yaml:"weather"      env:", prefix=WEATHER_"`
	Health       HealthConfig           `yaml:"health"       env:", prefix=HEALTH_"`
	Dashboard    Dashboard              `yaml:"dashboard"    env:", prefix=DASHBOARD_"`
	System       SystemConfig           `yaml:"system"       env:", prefix=SYSTEM_"`
}

// KubernetesConfig controls the cluster widget. With no kubeconfig path the
// in-cluster service account is used when running inside Kubernetes, then
// ~/.kube/config for local runs.
type KubernetesConfig struct {
	Kubeconfig      string `yaml:"kubeconfig"      env:"KUBECONFIG"`
	ArgocdNamespace string `yaml:"argocdNamespace" env:"ARGOCD_NAMESPACE"`
	Enabled         bool   `yaml:"enabled"         env:"ENABLED"`
}

// ProxmoxConfig controls the optional Proxmox VE cluster widget. APIToken is
// the full token value ("user@realm!tokenid=secret") of a read-only
// (PVEAuditor) API token. Insecure skips TLS verification for the stock
// self-signed node certificates.
type ProxmoxConfig struct {
	URL      string `yaml:"url"      env:"URL"`
	APIToken string `yaml:"apiToken" env:"API_TOKEN"`
	Enabled  bool   `yaml:"enabled"  env:"ENABLED"`
	Insecure bool   `yaml:"insecure" env:"INSECURE"`
}

// TrueNASConfig controls the optional TrueNAS widget (SCALE/Core REST v2).
type TrueNASConfig struct {
	URL      string `yaml:"url"      env:"URL"`
	APIKey   string `yaml:"apiKey"   env:"API_KEY"`
	Enabled  bool   `yaml:"enabled"  env:"ENABLED"`
	Insecure bool   `yaml:"insecure" env:"INSECURE"`
}

// PBSConfig controls the optional Proxmox Backup Server widget. APIToken is
// the full token value ("user@realm!tokenid:secret") with Datastore.Audit.
type PBSConfig struct {
	URL      string `yaml:"url"      env:"URL"`
	Node     string `yaml:"node"     env:"NODE"`
	APIToken string `yaml:"apiToken" env:"API_TOKEN"`
	Enabled  bool   `yaml:"enabled"  env:"ENABLED"`
	Insecure bool   `yaml:"insecure" env:"INSECURE"`
}

// CrowdSecConfig controls the optional CrowdSec decisions widget (LAPI
// bouncer API key).
type CrowdSecConfig struct {
	URL     string `yaml:"url"     env:"URL"`
	APIKey  string `yaml:"apiKey"  env:"API_KEY"`
	Enabled bool   `yaml:"enabled" env:"ENABLED"`
}

// ActionsConfig gates every write action (Argo sync, rollout restart,
// Proxmox guest start, Wake-on-LAN, Home Assistant toggles). Disabled by
// default; enabling it also needs the matching write credentials/RBAC.
type ActionsConfig struct {
	WakeOnLan []WakeHost `yaml:"wakeOnLan"`
	Enabled   bool       `yaml:"enabled"   env:"ENABLED"`
}

// WakeHost is one Wake-on-LAN target.
type WakeHost struct {
	Name      string `yaml:"name"`
	MAC       string `yaml:"mac"`
	Broadcast string `yaml:"broadcast"`
}

// PreferencesConfig controls server-side preference sync. Path defaults to
// sdash-prefs.json in the config directory; point it at writable storage.
type PreferencesConfig struct {
	Path    string `yaml:"path"    env:"PATH"`
	Enabled bool   `yaml:"enabled" env:"ENABLED"`
}

// GatusConfig controls the optional Gatus status widget.
type GatusConfig struct {
	URL     string `yaml:"url"     env:"URL"`
	Enabled bool   `yaml:"enabled" env:"ENABLED"`
}

// IntegrationsConfig holds per-application API credentials for the service
// card widgets. Keys live here (or in SDS__INTEGRATIONS_* env vars), never in
// dashboard.yaml, which is sent to browsers.
type IntegrationsConfig struct {
	Sonarr        IntegrationConfig `yaml:"sonarr"        env:", prefix=SONARR_"`
	Radarr        IntegrationConfig `yaml:"radarr"        env:", prefix=RADARR_"`
	Jellyfin      IntegrationConfig `yaml:"jellyfin"      env:", prefix=JELLYFIN_"`
	QBittorrent   IntegrationConfig `yaml:"qbittorrent"   env:", prefix=QBITTORRENT_"`
	Immich        IntegrationConfig `yaml:"immich"        env:", prefix=IMMICH_"`
	Miniflux      IntegrationConfig `yaml:"miniflux"      env:", prefix=MINIFLUX_"`
	Forgejo       IntegrationConfig `yaml:"forgejo"       env:", prefix=FORGEJO_"`
	Semaphore     IntegrationConfig `yaml:"semaphore"     env:", prefix=SEMAPHORE_"`
	Paperless     IntegrationConfig `yaml:"paperless"     env:", prefix=PAPERLESS_"`
	Syncthing     IntegrationConfig `yaml:"syncthing"     env:", prefix=SYNCTHING_"`
	HomeAssistant IntegrationConfig `yaml:"homeassistant" env:", prefix=HOMEASSISTANT_"`
}

// IntegrationConfig points at one upstream application's API. Username and
// Password are used instead of an API key by applications with form login
// (qBittorrent).
type IntegrationConfig struct {
	URL      string `yaml:"url"      env:"URL"`
	APIKey   string `yaml:"apiKey"   env:"API_KEY"`
	Username string `yaml:"username" env:"USERNAME"`
	Password string `yaml:"password" env:"PASSWORD"`
}

// Ready reports whether the integration has a URL and some credential.
func (integration IntegrationConfig) Ready() bool {
	if integration.URL == "" {
		return false
	}

	return integration.APIKey != "" || (integration.Username != "" && integration.Password != "")
}

// HealthConfig controls the background service prober.
type HealthConfig struct {
	Interval time.Duration `yaml:"interval" env:"INTERVAL"`
	Timeout  time.Duration `yaml:"timeout"  env:"TIMEOUT"`
	History  int           `yaml:"history"  env:"HISTORY"`
	Enabled  bool          `yaml:"enabled"  env:"ENABLED"`
}

// SystemConfig controls the host system-stats widget endpoint.
type SystemConfig struct {
	Enabled bool `yaml:"enabled" env:"ENABLED"`
}

// DockerConfig controls the optional Docker containers widget. Host accepts a
// unix socket path, a unix:/// or tcp:// URL, or empty to auto-detect the
// default socket locations.
type DockerConfig struct {
	Host    string `yaml:"host"    env:"HOST"`
	Enabled bool   `yaml:"enabled" env:"ENABLED"`
}

// WeatherConfig controls the optional Open-Meteo weather widget.
type WeatherConfig struct {
	Label     string  `yaml:"label"     env:"LABEL"`
	Units     string  `yaml:"units"     env:"UNITS"`
	Latitude  float64 `yaml:"latitude"  env:"LATITUDE"`
	Longitude float64 `yaml:"longitude" env:"LONGITUDE"`
	Enabled   bool    `yaml:"enabled"   env:"ENABLED"`
}

type WebConfig struct {
	BaseURL        string        `yaml:"baseUrl"        env:"BASE_URL"`
	Host           string        `yaml:"host"           env:"HOST"`
	AuthCookie     string        `yaml:"authCookie"     env:"AUTH_COOKIE"`
	CookieHashKey  string        `yaml:"cookieHashKey"  env:"COOKIE_HASH_KEY"`
	CookieBlockKey string        `yaml:"cookieBlockKey" env:"COOKIE_BLOCK_KEY"`
	Port           int           `yaml:"port"           env:"PORT"`
	SessionTimeout time.Duration `yaml:"sessionTimeout" env:"SESSION_TIMEOUT"`
}

type LogConfig struct {
	LevelStr  string     `yaml:"level"     env:"LEVEL"`
	Level     slog.Level `yaml:"-"`
	AddSource bool       `yaml:"addSource" env:"ADD_SOURCE"`
}

type Dashboard struct {
	Reload Reload `yaml:"reload" env:", prefix=RELOAD_"`
}

type Reload struct {
	PollInterval    time.Duration `yaml:"pollInterval"    env:"POLL_INTERVAL"`
	Enabled         bool          `yaml:"enabled"         env:"ENABLED"`
	ContinueOnError bool          `yaml:"continueOnError" env:"CONTINUE_ON_ERROR"`
	UseModTime      bool          `yaml:"useModTime"      env:"USE_MOD_TIME"`
	UseFsNotify     bool          `yaml:"useFsNotify"     env:"USE_FS_NOTIFY"`
}

type OidcConfig struct {
	ClientID          string `yaml:"clientId"          env:"CLIENT_ID"`
	ClientSecret      string `yaml:"clientSecret"      env:"CLIENT_SECRET"`
	DiscoveryEndpoint string `yaml:"discoveryEndpoint" env:"DISCOVERY_ENDPOINT"`
	Enabled           bool   `yaml:"enabled"           env:"ENABLED"`
	Default           bool   `yaml:"default"           env:"DEFAULT"`
}

func GetConfigWithContext(ctx context.Context) *Config {
	if config == nil {
		cfg := GetConfigFromFile(ctx)

		envConfig := new(envconfig.Config)
		envConfig.Target = cfg
		envConfig.Lookuper = envconfig.PrefixLookuper(EnvKeyPrefix, envconfig.OsLookuper())
		envConfig.DefaultOverwrite = true

		err := envconfig.ProcessWith(ctx, envConfig)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to process config from env", slog.Any("error", err))
			os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
		}

		if cfg.LogConfig.LevelStr != "" {
			level := slog.LevelInfo

			switch strings.ToLower(cfg.LogConfig.LevelStr) {
			case "debug":
				level = slog.LevelDebug
			case "info":
				level = slog.LevelInfo
			case "warn", "warning":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			default:
				slog.ErrorContext(
					ctx,
					"Invalid log level in config, defaulting to info",
					slog.String("level", cfg.LogConfig.LevelStr),
				)
			}

			cfg.LogConfig.Level = level
		}

		err = cfg.WebConfig.ValidateSessionTimeout()
		if err != nil {
			slog.ErrorContext(ctx, "Invalid session timeout", slog.Any("error", err))
			os.Exit(1) //nolint:forbidigo // Invalid security configuration must prevent startup.
		}

		// Validate the selected watcher after environment overrides, before a
		// non-positive interval can panic in time.NewTicker.
		if reload := cfg.Dashboard.Reload; reload.Enabled && !reload.UseFsNotify &&
			reload.PollInterval <= 0 {
			slog.ErrorContext(ctx, "Polling reload requires a positive pollInterval",
				slog.Duration("pollInterval", reload.PollInterval))
			os.Exit(1) //nolint:forbidigo // Invalid watcher configuration must prevent startup.
		}

		validateWidgetConfig(ctx, cfg)

		cfg.OidcConfig = processOidcConfigs(ctx, cfg.OidcConfig)
		config = cfg
	}

	return config
}

// validateWidgetConfig bounds the health prober settings and normalizes the
// weather units after environment overrides have been applied.
func validateWidgetConfig(ctx context.Context, cfg *Config) {
	if cfg.Health.Enabled {
		if cfg.Health.Interval <= 0 || cfg.Health.Timeout <= 0 {
			slog.ErrorContext(ctx, "Health checks require positive interval and timeout")
			os.Exit(1) //nolint:forbidigo // Invalid prober configuration must prevent startup.
		}

		if cfg.Health.History <= 0 || cfg.Health.History > maxHealthHistory {
			cfg.Health.History = defaultHealthHistory
		}
	}

	if cfg.Weather.Units != "imperial" {
		cfg.Weather.Units = "metric"
	}
}

func processOidcConfigs(
	ctx context.Context,
	oidcConfigs map[string]*OidcConfig,
) map[string]*OidcConfig {
	newAuthConfig := make(map[string]*OidcConfig, len(oidcConfigs))
	for key, val := range oidcConfigs {
		envKey := fmt.Sprintf("%sOIDC_%s_", EnvKeyPrefix, strings.ToUpper(key))
		val = getConfigFromEnv(ctx, val, envKey)
		newAuthConfig[key] = val
	}

	return newAuthConfig
}

func getConfigFromEnv[T any](ctx context.Context, config *T, envKey string) *T {
	if config == nil {
		return nil
	}

	envConfig := new(envconfig.Config)
	envConfig.Target = config
	envConfig.Lookuper = envconfig.PrefixLookuper(envKey, envconfig.OsLookuper())
	envConfig.DefaultOverwrite = true

	err := envconfig.ProcessWith(ctx, envConfig)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to process oidc config from env",
			slog.String("envKey", envKey),
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
	}

	return config
}

func GetConfigFromFile(ctx context.Context) *Config {
	newConfig := GetDefaultConfig()

	file, err := os.Open(getConfigFilePath(ctx))
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to open config file, using default config",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
	}
	// Require one complete mapping so startup can't silently ignore config data.
	decoder := yaml.NewDecoder(file)

	var document yaml.Node

	err = decoder.Decode(&document)
	switch {
	case errors.Is(err, io.EOF):
		err = ErrInvalidConfigDocument
	case err == nil:
		if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
			err = ErrInvalidConfigDocument
		} else {
			err = document.Decode(newConfig)
		}
	}

	if err == nil {
		err = decoder.Decode(new(yaml.Node))
		switch {
		case errors.Is(err, io.EOF):
			err = nil
		case err == nil:
			err = ErrInvalidConfigDocument
		}
	}

	closeErr := file.Close()
	if closeErr != nil {
		slog.ErrorContext(ctx, "Failed to close config file", slog.Any("error", closeErr))
	}

	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to decode config file",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
	}

	return newConfig
}

func getConfigFilePath(ctx context.Context) string {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath != "" {
		return configPath
	}

	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to get current working directory",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
		}

		configDir = cwd
	}

	configPath = path.Join(configDir, defaultConfigFile)

	return configPath
}

func GetDefaultConfig() *Config {
	cfg := new(Config)
	cfg.WebConfig.BaseURL = "http://localhost:" + strconv.Itoa(defaultPort)
	cfg.WebConfig.Port = defaultPort
	cfg.WebConfig.Host = "127.0.0.1"
	cfg.WebConfig.AuthCookie = "SDS__AUTH"
	cfg.WebConfig.SessionTimeout = defaultSessionTimeout
	cfg.LogConfig.Level = slog.LevelWarn
	cfg.LogConfig.AddSource = false
	cfg.Dashboard.Reload.Enabled = false
	cfg.Dashboard.Reload.ContinueOnError = false
	cfg.Dashboard.Reload.PollInterval = defaultPollInterval
	cfg.Dashboard.Reload.UseModTime = true
	cfg.Dashboard.Reload.UseFsNotify = false
	cfg.Health.Enabled = true
	cfg.Health.Interval = defaultHealthInterval
	cfg.Health.Timeout = defaultHealthTimeout
	cfg.Health.History = defaultHealthHistory
	cfg.System.Enabled = true
	cfg.Docker.Enabled = false
	cfg.Kubernetes.Enabled = false
	cfg.Kubernetes.ArgocdNamespace = "argocd"
	cfg.PBS.Node = "localhost"
	cfg.Preferences.Enabled = true
	cfg.Gatus.Enabled = false
	cfg.Weather.Enabled = false
	cfg.Weather.Units = "metric"

	return cfg
}

func (c *WebConfig) GetHashKey() []byte {
	hashKey := c.CookieHashKey

	var hashKeyByte []byte
	if hashKey != "" {
		hashKeyByte = []byte(hashKey)
	} else {
		hashKeyByte = securecookie.GenerateRandomKey(defaultCookieHashKeySize)
		if hashKeyByte == nil {
			slog.ErrorContext(context.Background(), "Failed to generate hash key")
			os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
		}
	}

	return hashKeyByte
}

func (c *WebConfig) GetBlockKey() []byte {
	blockKey := c.CookieBlockKey

	var blockKeyByte []byte
	if blockKey != "" {
		blockKeyByte = []byte(blockKey)
	} else {
		blockKeyByte = securecookie.GenerateRandomKey(defaultCookieBlockKeySize)
		if blockKeyByte == nil {
			slog.ErrorContext(context.Background(), "Failed to generate block key")
			os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
		}
	}

	return blockKeyByte
}

// ValidateSessionTimeout bounds session and browser cookie lifetimes to the
// secure-cookie codec's maximum age and HTTP cookie second precision.
func (c *WebConfig) ValidateSessionTimeout() error {
	if c.SessionTimeout < time.Second || c.SessionTimeout > maxSessionTimeout ||
		c.SessionTimeout%time.Second != 0 {
		return ErrInvalidSessionTimeout
	}

	return nil
}
