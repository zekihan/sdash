package config

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path"
	"sync"
)

const dashboardConfigFile = "dashboard.yaml"

var (
	dashboardMutex  sync.Mutex
	dashboardConfig *DashboardConfig
)

type DashboardConfig struct {
	Services []DashboardService `yaml:"services"`
}

type DashboardService struct {
	Name  string   `yaml:"name"`
	Title string   `yaml:"title"`
	URL   string   `yaml:"url"`
	Icon  string   `yaml:"icon"`
	Tags  []string `yaml:"tags"`
}

func GetDashboardConfig() *DashboardConfig {
	dashboardMutex.Lock()
	defer dashboardMutex.Unlock()
	if dashboardConfig == nil {
		cfg := GetDashboardConfigFromFile()
		ctx := context.Background()
		if err := envconfig.Process(ctx, cfg); err != nil {
			log.Fatal(err)
		}
		dashboardConfig = cfg
	}
	return dashboardConfig
}

func GetDashboardConfigFromFile() *DashboardConfig {
	config := &DashboardConfig{}
	file, err := os.Open(getDashboardConfigFilePath())
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
				dashboardMutex.Lock()
				dashboardConfig = GetDashboardConfigFromFile()
				dashboardMutex.Unlock()
			}
		case err := <-watcher.Errors:
			log.Printf("Watcher error: %v", err)
		}
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
