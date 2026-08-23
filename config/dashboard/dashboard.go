//nolint:forbidigo // This package configures log packages.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"

	generalConfig "github.com/zekihan/sdash/config"
)

const configFile = "dashboard.yaml"

var (
	mutex  sync.RWMutex
	config *Config
)

var ErrConfigFileIsDirectory = errors.New("config file is a directory")

type (
	GroupList    []*Group
	ServicesList []*Service
)

type Config struct {
	LastModTime      time.Time           `yaml:"-"`
	Services         map[string]*Service `yaml:"services"`
	Groups           map[string]*Group   `yaml:"groups"`
	SearchConfig     *SearchConfig       `yaml:"search"`
	OrderServicesVia Ordering            `yaml:"orderServicesVia"`
	OrderGroupsVia   Ordering            `yaml:"orderGroupsVia"`
	ServicesList     ServicesList        `yaml:"-"`
	GroupList        GroupList           `yaml:"-"`
}

func GetConfigWithContext(ctx context.Context) *Config {
	if getCurrentConfig(ctx) == nil {
		reload(ctx)
	}

	return getCurrentConfig(ctx)
}

func getCurrentConfig(_ context.Context) *Config {
	mutex.RLock()
	defer mutex.RUnlock()

	return config
}

func getConfigFromFile(ctx context.Context) (*Config, error) {
	newConfig := new(Config)
	configPath := getConfigFilePath(ctx)

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return newConfig, fmt.Errorf("failed to stat config file: %w", err)
	}

	if fileInfo.IsDir() {
		return newConfig, ErrConfigFileIsDirectory
	}

	currMod := fileInfo.ModTime()
	newConfig.LastModTime = currMod

	file, err := os.Open(configPath)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to open config file",
			slog.Any("error", err),
		)

		return newConfig, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func(file *os.File) {
		fileErr := file.Close()
		if fileErr != nil {
			slog.ErrorContext(
				ctx,
				"Failed to close config file",
				slog.Any("error", fileErr),
			)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to read config file",
			slog.Any("error", err),
		)

		return newConfig, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(content, newConfig)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to parse config file",
			slog.Any("error", err),
		)

		return newConfig, fmt.Errorf("failed to parse config file: %w", err)
	}

	newConfig.postProcessConfig()

	return newConfig, nil
}

func (cfg *Config) postProcessConfig() {
	cfg.postProcessGroups()
	cfg.postProcessServices()
	cfg.postProcessSearchConfig()
}

func WatchConfigFile() {
	ctx := context.Background()

	if generalConfig.GetConfigWithContext(ctx).Dashboard.Reload.Enabled {
		if generalConfig.GetConfigWithContext(ctx).Dashboard.Reload.UseFsNotify {
			watchConfigFileViaFsNotify(ctx)
		} else {
			watchConfigFileViaModTime(ctx)
		}
	}
}

func watchConfigFileViaModTime(ctx context.Context) {
	ticker := time.NewTicker(generalConfig.GetConfigWithContext(ctx).Dashboard.Reload.PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		configPath := getConfigFilePath(ctx)

		var lastMod time.Time

		cd := getCurrentConfig(ctx)
		if cd == nil {
			lastMod = time.Now()
		} else {
			lastMod = cd.LastModTime
		}

		fileInfo, err := os.Stat(configPath)
		if err != nil || fileInfo.IsDir() {
			continue
		}

		currMod := fileInfo.ModTime()
		if currMod.IsZero() {
			continue
		}

		if lastMod.Unix() < currMod.Unix() {
			slog.InfoContext(
				ctx,
				"Config file changed",
				slog.Any("lastModified", lastMod),
				slog.Any("currentModified", currMod),
				slog.String("path", configPath),
			)
			reload(ctx)
		}
	}
}

func watchConfigFileViaFsNotify(ctx context.Context) {
	configPath := getConfigFilePath(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to watch config file",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:forbidigo // Using os.Exit in config initialization is acceptable.
	}
	defer func(watcher *fsnotify.Watcher) {
		watcherErr := watcher.Close()
		if watcherErr != nil {
			slog.ErrorContext(
				ctx,
				"Failed to close watcher",
				slog.Any("error", watcherErr),
			)
		}
	}(watcher)

	err = watcher.AddWith(configPath)
	if err != nil {
		watcherErr := watcher.Close()
		if watcherErr != nil {
			slog.ErrorContext(
				ctx,
				"Failed to close watcher after add error",
				slog.Any("error", watcherErr),
			)
		}

		slog.ErrorContext(
			ctx,
			"Failed to watch config file",
			slog.Any("error", err),
		)
		//nolint:forbidigo,gocritic // Using os.Exit in config initialization is acceptable.
		os.Exit(1)
	}

	for {
		slog.DebugContext(
			ctx,
			"Config file changed",
			slog.String("path", configPath),
		)

		select {
		case event := <-watcher.Events:
			slog.DebugContext(
				ctx,
				"Config file changed",
				slog.Any("event", event),
				slog.String("path", configPath),
			)

			if event.Has(fsnotify.Write) {
				slog.InfoContext(
					ctx,
					"Config file modified, reloading",
					slog.String("path", configPath),
				)
				reload(ctx)
			}
		case err := <-watcher.Errors:
			slog.ErrorContext(
				ctx,
				"Watcher error",
				slog.Any("error", err),
				slog.String("path", configPath),
			)
		}
	}
}

func reload(ctx context.Context) {
	mutex.Lock()
	defer mutex.Unlock()

	configFromFile, err := getConfigFromFile(ctx)
	if err == nil {
		config = configFromFile

		return
	}

	if !generalConfig.GetConfigWithContext(ctx).Dashboard.Reload.ContinueOnError {
		panic(err)
	}
}

func getConfigFilePath(ctx context.Context) string {
	configPath := os.Getenv("DASHBOARD_CONFIG_PATH")
	if configPath != "" {
		return configPath
	}

	configDir := os.Getenv("DASHBOARD_CONFIG_DIR")
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

	configPath = path.Join(configDir, configFile)

	return configPath
}
