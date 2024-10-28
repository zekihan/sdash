package config

import (
	"context"
	"fmt"
	"github.com/gorilla/securecookie"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

const EnvKeyPrefix = "SDS__"
const defaultConfigFile = "config.yaml"

var config *Config

type Config struct {
	WebConfig  WebConfig              `yaml:"web" env:", prefix=WEB_"`
	LogConfig  LogConfig              `yaml:"log" env:", prefix=LOG_"`
	Dashboard  Dashboard              `yaml:"dashboard" env:", prefix=DASHBOARD_"`
	OidcConfig map[string]*OidcConfig `yaml:"oidc"`
}

type WebConfig struct {
	BaseURL        string        `yaml:"base_url" env:"BASE_URL"`
	Port           int           `yaml:"port" env:"PORT"`
	Host           string        `yaml:"host" env:"HOST"`
	AuthCookie     string        `yaml:"auth_cookie" env:"AUTH_COOKIE"`
	SessionTimeout time.Duration `yaml:"session_timeout" env:"SESSION_TIMEOUT"`
	CookieHashKey  string        `yaml:"cookie_hash_key" env:"COOKIE_HASH_KEY"`
	CookieBlockKey string        `yaml:"cookie_block_key" env:"COOKIE_BLOCK_KEY"`
}

type LogConfig struct {
	Level         logrus.Level `yaml:"-"`
	LevelStr      string       `yaml:"level" env:"LEVEL"`
	Path          string       `yaml:"path" env:"PATH"`
	AddMethodName bool         `yaml:"add_method_name" env:"ADD_METHOD_NAME"`
}

type Dashboard struct {
	Reload Reload `yaml:"reload" env:", prefix=RELOAD_"`
}

type Reload struct {
	Enabled         bool          `yaml:"enabled" env:"ENABLED"`
	ContinueOnError bool          `yaml:"continue_on_error" env:"CONTINUE_ON_ERROR"`
	PollInterval    time.Duration `yaml:"poll_interval" env:"POLL_INTERVAL"`
	UseModTime      bool          `yaml:"use_mod_time" env:"USE_MOD_TIME"`
	UseFsNotify     bool          `yaml:"use_fs_notify" env:"USE_FS_NOTIFY"`
}

type OidcConfig struct {
	Enabled           bool   `yaml:"enabled" env:"ENABLED"`
	Default           bool   `yaml:"default" env:"DEFAULT"`
	ClientID          string `yaml:"client_id" env:"CLIENT_ID"`
	ClientSecret      string `yaml:"client_secret" env:"CLIENT_SECRET"`
	DiscoveryEndpoint string `yaml:"discovery_endpoint" env:"DISCOVERY_ENDPOINT"`
}

func GetConfig() *Config {
	if config == nil {
		cfg := GetConfigFromFile()
		ctx := context.Background()
		if err := envconfig.ProcessWith(ctx, &envconfig.Config{
			Target:   cfg,
			Lookuper: envconfig.PrefixLookuper(EnvKeyPrefix, envconfig.OsLookuper()),
		}); err != nil {
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
		return config
	}
	ctx := context.Background()
	if err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   config,
		Lookuper: envconfig.PrefixLookuper(envKey, envconfig.OsLookuper()),
	}); err != nil {
		log.Fatalf("Failed to process config from env: %v", err)
	}
	return config
}

func GetConfigFromFile() *Config {
	config := GetDefaultConfig()
	file, err := os.Open(getConfigFilePath())
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	err = yaml.Unmarshal(content, config)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
	return config
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
			BaseURL:        "http://localhost:8080",
			Port:           8080,
			Host:           "127.0.0.1",
			AuthCookie:     "SDS__AUTH",
			SessionTimeout: 24 * time.Hour,
		},
		LogConfig: LogConfig{
			Level:         logrus.WarnLevel,
			AddMethodName: false,
		},
		Dashboard: Dashboard{
			Reload: Reload{
				Enabled:         false,
				ContinueOnError: false,
				PollInterval:    5 * time.Second,
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
		hashKeyByte = securecookie.GenerateRandomKey(64)
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
		blockKeyByte = securecookie.GenerateRandomKey(32)
		if blockKeyByte == nil {
			log.Fatal("Failed to generate block key")
		}
	}
	return blockKeyByte
}
