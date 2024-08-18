package config

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
	"os"
	"path"
	"slices"
	"sort"
	"sync"
	"time"
)

const dashboardConfigFile = "dashboard.yaml"

var (
	dashboardMutex  sync.RWMutex
	dashboardConfig *DashboardConfig
)

type DashboardConfig struct {
	Services     map[string]DashboardService `yaml:"services"`
	ServicesList []DashboardService          `yaml:"-"`
	GroupList    []string                    `yaml:"-"`
	lastModTime  time.Time                   `yaml:"-"`
}

func (c DashboardConfig) getServices() []DashboardService {
	keys := make([]string, 0)
	for k := range c.Services {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	services := make([]DashboardService, 0)
	for _, k := range keys {
		service := c.Services[k]
		service.Key = k
		services = append(services, service)
	}
	return services
}

func (c DashboardConfig) GetGroupsFromList() []string {
	groups := make([]string, 0)
	for _, service := range c.ServicesList {
		if service.Group == "" {
			continue
		}
		if slices.Contains(groups, service.Group) {
			continue
		}
		groups = append(groups, service.Group)
	}
	sort.Strings(groups)
	return groups
}

func (c DashboardConfig) getGroupsMap() []string {
	groups := make([]string, 0)
	for _, service := range c.Services {
		if service.Group == "" {
			continue
		}
		if slices.Contains(groups, service.Group) {
			continue
		}
		groups = append(groups, service.Group)
	}
	sort.Strings(groups)
	return groups
}

type DashboardService struct {
	Key       string   `yaml:"-"`
	Title     string   `yaml:"title"`
	Group     string   `yaml:"group"`
	Location  string   `yaml:"location"`
	Cluster   string   `yaml:"cluster"`
	URL       string   `yaml:"url"`
	Icon      string   `yaml:"icon"`
	ProxyIcon bool     `yaml:"proxy_icon"`
	Tags      []string `yaml:"tags"`
}

func (s *DashboardService) GetIconURL() string {
	if !s.ProxyIcon {
		return s.Icon
	}
	return fmt.Sprintf("/icon?url=%s", url.QueryEscape(s.Icon))
}

func GetDashboardConfig() *DashboardConfig {
	if getCurrentDashboard() == nil {
		reloadDashboard()
	}
	return getCurrentDashboard()
}

func getCurrentDashboard() *DashboardConfig {
	dashboardMutex.RLock()
	defer dashboardMutex.RUnlock()
	return dashboardConfig
}

func getDashboardConfigFromFile() (*DashboardConfig, error) {
	config := &DashboardConfig{}
	configPath := getDashboardConfigFilePath()
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
	for k, v := range config.Services {
		v.Key = k
		config.Services[k] = v
		if v.Tags == nil {
			v.Tags = make([]string, 0)
		}
		if v.Group == "" {
			v.Group = "Others"
		}
	}
	config.ServicesList = config.getServices()
	config.GroupList = config.getGroupsMap()
	return config, nil
}

func WatchConfigFile() {
	if GetConfig().Dashboard.Reload.Enabled {
		if GetConfig().Dashboard.Reload.UseFsNotify {
			WatchConfigFileViaFsNotify()
		} else {
			WatchConfigFileViaModTime()
		}
	}
}

func WatchConfigFileViaModTime() {
	for {
		time.Sleep(GetConfig().Dashboard.Reload.PollInterval)
		configPath := getDashboardConfigFilePath()
		var lastMod time.Time
		cd := getCurrentDashboard()
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
			reloadDashboard()
		}
	}
}

func WatchConfigFileViaFsNotify() {
	configPath := getDashboardConfigFilePath()
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
				reloadDashboard()
			}
		case err := <-watcher.Errors:
			log.Printf("Watcher error: %v", err)
		}
	}
}

func reloadDashboard() {
	dashboardMutex.Lock()
	defer dashboardMutex.Unlock()
	dashboardConfigFromFile, err := getDashboardConfigFromFile()
	if err == nil {
		dashboardConfig = dashboardConfigFromFile
		return
	}
	if !GetConfig().Dashboard.Reload.ContinueOnError {
		panic(err)
	}
}

func getDashboardConfigFilePath() string {
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
	configPath = path.Join(configDir, dashboardConfigFile)
	return configPath
}
