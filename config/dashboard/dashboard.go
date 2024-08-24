package dashboard

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	cfg "github.com/zekihan/sdash/config"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path"
	"sync"
	"time"
)

const configFile = "dashboard.yaml"

var (
	mutex  sync.RWMutex
	config *Config
)

type GroupList []*Group
type ServicesList []*Service

type Config struct {
	Services         map[string]*Service `yaml:"services"`
	ServicesList     ServicesList        `yaml:"-"`
	OrderServicesVia Ordering            `yaml:"order_services_via"`
	Groups           map[string]*Group   `yaml:"groups"`
	GroupList        GroupList           `yaml:"-"`
	OrderGroupsVia   Ordering            `yaml:"order_groups_via"`
	lastModTime      time.Time           `yaml:"-"`
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
	config := &Config{}
	configPath := getConfigFilePath()
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return config, err
	}
	if fileInfo.IsDir() {
		return config, errors.New("config file is a directory")
	}
	currMod := fileInfo.ModTime()
	config.lastModTime = currMod
	file, err := os.Open(configPath)
	if err != nil {
		log.Errorf("Failed to open config file: %v", err)
		return config, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Failed to read config file: %v", err)
		return config, err
	}
	err = yaml.Unmarshal(content, config)
	if err != nil {
		log.Errorf("Failed to parse config file: %v", err)
		return config, err
	}
	config.postProcessConfig()
	return config, nil
}

func (c *Config) postProcessConfig() {
	c.postProcessGroups()
	c.postProcessServices()
}

func WatchConfigFile() {
	if cfg.GetConfig().Dashboard.Reload.Enabled {
		if cfg.GetConfig().Dashboard.Reload.UseFsNotify {
			watchConfigFileViaFsNotify()
		} else {
			watchConfigFileViaModTime()
		}
	}
}

func watchConfigFileViaModTime() {
	for {
		time.Sleep(cfg.GetConfig().Dashboard.Reload.PollInterval)
		configPath := getConfigFilePath()
		var lastMod time.Time
		cd := getCurrentConfig()
		if cd == nil {
			lastMod = time.Now()
		} else {
			lastMod = cd.lastModTime
		}
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			return
		}
		if fileInfo.IsDir() {
			return
		}
		currMod := fileInfo.ModTime()
		if currMod.IsZero() {
			return
		}
		if lastMod.Unix() < currMod.Unix() {
			log.Printf("Config file changed: %s", configPath)
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
	defer watcher.Close()
	err = watcher.AddWith(configPath)
	if err != nil {
		log.Fatalf("Failed to watch config file: %v", err)
	}
	for {
		log.Debugf("Watching config file: %s", configPath)
		select {
		case event := <-watcher.Events:
			log.Debugf("Config file changed. Event: %v", event)
			if event.Has(fsnotify.Write) {
				log.Printf("Config file changed: %s", event.Name)
				reload()
			}
		case err := <-watcher.Errors:
			log.Printf("Watcher error: %v", err)
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
	if !cfg.GetConfig().Dashboard.Reload.ContinueOnError {
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
