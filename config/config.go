//nolint:forbidigo // This package configures log packages.
package config

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
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
	LevelStr      string       `yaml:"level"         env:"LEVEL"`
	Path          string       `yaml:"path"          env:"PATH"`
	Level         logrus.Level `yaml:"-"`
	AddMethodName bool         `yaml:"addMethodName" env:"ADD_METHOD_NAME"`
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

func GetConfig() *Config {
	if config == nil {
		cfg := GetConfigFromFile()

		ctx := context.Background()

		err := envconfig.ProcessWith(ctx, &envconfig.Config{
			Target:           cfg,
			Lookuper:         envconfig.PrefixLookuper(EnvKeyPrefix, envconfig.OsLookuper()),
			DefaultOverwrite: true,
		})
		if err != nil {
			log.Fatalf("Failed to process env config: %v", err)
		}

		if cfg.LogConfig.LevelStr != "" {
			level, err := logrus.ParseLevel(cfg.LogConfig.LevelStr)
			if err != nil {
				log.Fatalf("Failed to parse log level: %v", err)
			}

			cfg.LogConfig.Level = level
		}

		cfg.OidcConfig = processOidcConfigs(cfg.OidcConfig)
		config = cfg
	}

	return config
}

func processOidcConfigs(oidcConfigs map[string]*OidcConfig) map[string]*OidcConfig {
	newAuthConfig := make(map[string]*OidcConfig, len(oidcConfigs))
	for key, val := range oidcConfigs {
		envKey := fmt.Sprintf("%sOIDC_%s_", EnvKeyPrefix, strings.ToUpper(key))
		val = getConfigFromEnv(val, envKey)
		newAuthConfig[key] = val
	}

	return newAuthConfig
}

func getConfigFromEnv[T any](config *T, envKey string) *T {
	if config == nil {
		return nil
	}

	ctx := context.Background()

	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:           config,
		Lookuper:         envconfig.PrefixLookuper(envKey, envconfig.OsLookuper()),
		DefaultOverwrite: true,
	})
	if err != nil {
		log.Fatalf("Failed to process config from env: %v", err)
	}

	return config
}

func GetConfigFromFile() *Config {
	newConfig := GetDefaultConfig()

	file, err := os.Open(getConfigFilePath())
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
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

		log.Fatalf("Failed to read config file: %v", err) //nolint:gocritic
	}

	err = yaml.Unmarshal(content, newConfig)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	return newConfig
}

func getConfigFilePath() string {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath != "" {
		return configPath
	}

	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("%v", err)
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
			Level:         logrus.WarnLevel,
			AddMethodName: false,
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
			log.Fatal("Failed to generate hash key")
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
			log.Fatal("Failed to generate block key")
		}
	}

	return blockKeyByte
}
