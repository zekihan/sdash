package config

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path"
	"slices"
	"sort"
	"sync"
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
}

func (c DashboardConfig) GetServices() []DashboardService {
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

func (c DashboardConfig) GetGroupsMap() []string {
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
	Key      string   `yaml:"-"`
	Title    string   `yaml:"title"`
	Group    string   `yaml:"group"`
	Location string   `yaml:"location"`
	Cluster  string   `yaml:"cluster"`
	URL      string   `yaml:"url"`
	Icon     string   `yaml:"icon"`
	Tags     []string `yaml:"tags"`
}

func GetDashboardConfig() *DashboardConfig {
	if GetCurrentDashboard() == nil {
		reloadDashboard()
	}
	return GetCurrentDashboard()
}

func GetCurrentDashboard() *DashboardConfig {
	dashboardMutex.RLock()
	defer dashboardMutex.RUnlock()
	return dashboardConfig
}

func GetDashboardConfigFromFile() (*DashboardConfig, error) {
	config := &DashboardConfig{}
	file, err := os.Open(getDashboardConfigFilePath())
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
	config.ServicesList = config.GetServices()
	config.GroupList = config.GetGroupsMap()
	return config, nil
}

func WatchConfigFile() {
	configPath := getDashboardConfigFilePath()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()
	err = watcher.Add(configPath)
	if err != nil {
		log.Fatalf("Failed to watch config file: %v", err)
	}
	for {
		select {
		case event := <-watcher.Events:
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
	dashboardConfigFromFile, err := GetDashboardConfigFromFile()
	if err == nil {
		dashboardConfig = dashboardConfigFromFile
		return
	}
	if !GetConfig().ShouldContinueOnDashboardReloadError {
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
