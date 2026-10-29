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
	"path/filepath"
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

var (
	ErrConfigFileIsDirectory = errors.New("config file is a directory")
	ErrEmptyConfig           = errors.New(
		"config must contain a YAML mapping; use {} for an empty dashboard",
	)
)

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

	fileInfo, err := file.Stat()
	if err != nil {
		return newConfig, fmt.Errorf("failed to stat config file: %w", err)
	}

	if fileInfo.IsDir() {
		return newConfig, ErrConfigFileIsDirectory
	}

	content, err := io.ReadAll(file)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to read config file",
			slog.Any("error", err),
		)

		return newConfig, fmt.Errorf("failed to read config file: %w", err)
	}

	// A truncated file, comments-only document, or YAML null must not publish
	// an empty dashboard while an editor is still writing. An explicit mapping
	// (including {}) is required; errors retain the existing reload policy.
	var parsedConfig *Config

	err = yaml.Unmarshal(content, &parsedConfig)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to parse config file",
			slog.Any("error", err),
		)

		return newConfig, fmt.Errorf("failed to parse config file: %w", err)
	}

	if parsedConfig == nil {
		return newConfig, ErrEmptyConfig
	}

	parsedConfig.LastModTime = fileInfo.ModTime()
	parsedConfig.postProcessConfig()

	return parsedConfig, nil
}

func (cfg *Config) postProcessConfig() {
	cfg.ensureDefaultGroup()
	cfg.postProcessServices()
	cfg.postProcessGroups()
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reloadIfModTimeChanged(ctx)
		}
	}
}

func reloadIfModTimeChanged(ctx context.Context) {
	configPath := getConfigFilePath(ctx)

	fileInfo, err := os.Stat(configPath)
	if err != nil || fileInfo.IsDir() || fileInfo.ModTime().IsZero() {
		return
	}

	current := getCurrentConfig(ctx)
	// Compare the full timestamp in both directions: replacements may be older.
	if current == nil || !current.LastModTime.Equal(fileInfo.ModTime()) {
		reload(ctx)
	}
}

func watchConfigFileViaFsNotify(ctx context.Context) {
	configPath := filepath.Clean(getConfigFilePath(ctx))

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

	err = watcher.AddWith(filepath.Dir(configPath))
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

	watchConfigEvents(ctx, watcher, configPath)
}

func watchConfigEvents(ctx context.Context, watcher *fsnotify.Watcher, configPath string) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if filepath.Clean(event.Name) == configPath &&
				event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				reload(ctx)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

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
