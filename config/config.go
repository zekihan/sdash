//nolint:forbidigo // This package configures log packages.
package config

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

const (
	EnvKeyPrefix              = "SDS__"
	defaultConfigFile         = "config.yaml"
	defaultPort               = 8080
	defaultSessionTimeout     = 24 * time.Hour
	defaultPollInterval       = 5 * time.Second
	defaultCookieHashKeySize  = 64
	defaultCookieBlockKeySize = 32
)

var config *Config

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

		err := envconfig.ProcessWith(ctx, &envconfig.Config{
			Target:           cfg,
			Lookuper:         envconfig.PrefixLookuper(EnvKeyPrefix, envconfig.OsLookuper()),
			DefaultOverwrite: true,
		})
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

	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:           config,
		Lookuper:         envconfig.PrefixLookuper(envKey, envconfig.OsLookuper()),
		DefaultOverwrite: true,
	})
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
			slog.Any("error", fileErr),
		)
		//nolint:forbidigo,gocritic // Using os.Exit in config initialization is acceptable.
		os.Exit(1)
	}

	err = yaml.Unmarshal(content, newConfig)
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
	return &Config{
		WebConfig: WebConfig{
			BaseURL:        "http://localhost:" + strconv.Itoa(defaultPort),
			Port:           defaultPort,
			Host:           "127.0.0.1",
			AuthCookie:     "SDS__AUTH",
			SessionTimeout: defaultSessionTimeout,
		},
		LogConfig: LogConfig{
			Level:     slog.LevelWarn,
			AddSource: false,
		},
		Dashboard: Dashboard{
			Reload: Reload{
				Enabled:         false,
				ContinueOnError: false,
				PollInterval:    defaultPollInterval,
				UseModTime:      true,
				UseFsNotify:     false,
			},
		},
	}
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
