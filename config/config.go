//nolint:forbidigo // This package configures log packages.
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
)

var config *Config

var (
	ErrInvalidSessionTimeout = errors.New(
		"sessionTimeout must be a whole number of seconds between 1s and 720h",
	)
	ErrInvalidConfigDocument = errors.New(
		"config must contain a YAML mapping; use {} for defaults",
	)
)

type Config struct {
	OidcConfig map[string]*OidcConfig `yaml:"oidc"`
	LogConfig  LogConfig              `yaml:"log"       env:", prefix=LOG_"`
	WebConfig  WebConfig              `yaml:"web"       env:", prefix=WEB_"`
	Dashboard  Dashboard              `yaml:"dashboard" env:", prefix=DASHBOARD_"`
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

		cfg.OidcConfig = processOidcConfigs(ctx, cfg.OidcConfig)
		config = cfg
	}

	return config
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
	defer func(file *os.File) {
		fileErr := file.Close()
		if fileErr != nil {
			fmt.Printf("Failed to close config file: %v", fileErr)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		fileErr := file.Close()
		if fileErr != nil {
			fmt.Printf("Failed to close config file after read error: %v", fileErr)
		}

		slog.ErrorContext(
			ctx,
			"Failed to read config file, using default config",
			slog.Any("error", err),
		)
		//nolint:forbidigo,gocritic // Using os.Exit in config initialization is acceptable.
		os.Exit(1)
	}

	// Require an explicit mapping so an empty or truncated file cannot silently
	// start the server with defaults and no authentication providers.
	var document yaml.Node

	err = yaml.Unmarshal(content, &document)
	if err == nil {
		if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
			err = ErrInvalidConfigDocument
		} else {
			err = document.Decode(newConfig)
		}
	}

	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to unmarshal config file, using default config",
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
