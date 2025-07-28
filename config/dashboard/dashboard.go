//nolint:forbidigo // This package configures log packages.
package dashboard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"

	log "github.com/sirupsen/logrus"

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

func GetConfig() *Config {
	if getCurrentConfig() == nil {
		reload()
	}

	return getCurrentConfig()
}

func getCurrentConfig() *Config {
	mutex.RLock()
	defer mutex.RUnlock()

	return config
}

func getConfigFromFile() (*Config, error) {
	newConfig := &Config{}
	configPath := getConfigFilePath()

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
		log.Errorf("Failed to open config file: %v", err)

		return newConfig, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func(file *os.File) {
		fileErr := file.Close()
		if fileErr != nil {
			log.Errorf("Failed to close config file: %v", fileErr)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Failed to read config file: %v", err)

		return newConfig, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(content, newConfig)
	if err != nil {
		log.Errorf("Failed to parse config file: %v", err)

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
	if generalConfig.GetConfig().Dashboard.Reload.Enabled {
		if generalConfig.GetConfig().Dashboard.Reload.UseFsNotify {
			watchConfigFileViaFsNotify()
		} else {
			watchConfigFileViaModTime()
		}
	}
}

func watchConfigFileViaModTime() {
	ticker := time.NewTicker(generalConfig.GetConfig().Dashboard.Reload.PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		configPath := getConfigFilePath()

		var lastMod time.Time

		cd := getCurrentConfig()
		if cd == nil {
			lastMod = time.Now()
		} else {
			lastMod = cd.LastModTime
		}

		fileInfo, err := os.Stat(configPath)
		if err != nil || fileInfo.IsDir() {
			return
		}

		currMod := fileInfo.ModTime()
		if currMod.IsZero() {
			return
		}

		if lastMod.Unix() < currMod.Unix() {
			log.Infof("Config file changed: %s", configPath)
			reload()
		}
	}
}

func watchConfigFileViaFsNotify() {
	configPath := getConfigFilePath()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		watcherErr := watcher.Close()
		if watcherErr != nil {
			log.Errorf("Failed to close watcher: %v", watcherErr)
		}
	}(watcher)

	err = watcher.AddWith(configPath)
	if err != nil {
		watcherErr := watcher.Close()
		if watcherErr != nil {
			log.Errorf("Failed to close watcher: %v", watcherErr)
		}

		log.Fatalf( //nolint:gocritic // closed above
			"Failed to watch config file: %v",
			err,
		)
	}

	for {
		log.Debugf("Watching config file: %s", configPath)

		select {
		case event := <-watcher.Events:
			log.Debugf("Config file changed. Event: %v", event)

			if event.Has(fsnotify.Write) {
				log.Infof("Config file changed: %s", event.Name)
				reload()
			}
		case err := <-watcher.Errors:
			log.Infof("Watcher error: %v", err)
		}
	}
}

func reload() {
	mutex.Lock()
	defer mutex.Unlock()

	configFromFile, err := getConfigFromFile()
	if err == nil {
		config = configFromFile

		return
	}

	if !generalConfig.GetConfig().Dashboard.Reload.ContinueOnError {
		panic(err)
	}
}

func getConfigFilePath() string {
	configPath := os.Getenv("DASHBOARD_CONFIG_PATH")
	if configPath != "" {
		return configPath
	}

	configDir := os.Getenv("DASHBOARD_CONFIG_DIR")
	if configDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("%v", err)
		}

		configDir = cwd
	}

	configPath = path.Join(configDir, configFile)

	return configPath
}
