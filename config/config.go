package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path"
	"time"
)

const defaultConfigFile = "config.yaml"

var config *Config

type Config struct {
	WebConfig WebConfig `yaml:"web" env:", prefix=SDS__WEB_"`
	LogConfig LogConfig `yaml:"log" env:", prefix=SDS__LOG_"`
	Dashboard Dashboard `yaml:"dashboard" env:", prefix=SDS__DASHBOARD_"`
}

type WebConfig struct {
	Port int    `yaml:"port" env:"PORT"`
	Host string `yaml:"host" env:"HOST"`
}

type LogConfig struct {
	Level         logrus.Level `yaml:"-"`
	LevelStr      string       `yaml:"level" env:"LEVEL"`
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

func GetConfig() *Config {
	if config == nil {
		cfg := GetConfigFromFile()
		ctx := context.Background()
		if err := envconfig.Process(ctx, cfg); err != nil {
			log.Fatal(err)
		}
		if cfg.LogConfig.LevelStr != "" {
			level, err := logrus.ParseLevel(cfg.LogConfig.LevelStr)
			if err != nil {
				log.Fatalf("Failed to parse log level: %v", err)
			}
			cfg.LogConfig.Level = level
		}
		config = cfg
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
			Port: 8080,
			Host: "",
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
