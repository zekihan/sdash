package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()
	if cfg.WebConfig.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.WebConfig.Port)
	}
	if cfg.WebConfig.BaseURL != "http://localhost:8080" {
		t.Errorf("expected default BaseURL, got %s", cfg.WebConfig.BaseURL)
	}
	if cfg.LogConfig.Level != 3 { // logrus.WarnLevel == 3
		t.Errorf("expected default log level 3, got %d", cfg.LogConfig.Level)
	}
	if cfg.Dashboard.Reload.PollInterval != 5*time.Second {
		t.Errorf("expected default poll interval 5s, got %v", cfg.Dashboard.Reload.PollInterval)
	}
}

func TestWebConfig_GetHashKey_GeneratesKey(t *testing.T) {
	wc := WebConfig{}
	key := wc.GetHashKey()
	if len(key) != 64 {
		t.Errorf("expected hash key length 64, got %d", len(key))
	}
}

func TestWebConfig_GetHashKey_UsesProvidedKey(t *testing.T) {
	wc := WebConfig{CookieHashKey: "myhashkey"}
	key := wc.GetHashKey()
	if string(key) != "myhashkey" {
		t.Errorf("expected hash key 'myhashkey', got %s", string(key))
	}
}

func TestWebConfig_GetBlockKey_GeneratesKey(t *testing.T) {
	wc := WebConfig{}
	key := wc.GetBlockKey()
	if len(key) != 32 {
		t.Errorf("expected block key length 32, got %d", len(key))
	}
}

func TestWebConfig_GetBlockKey_UsesProvidedKey(t *testing.T) {
	wc := WebConfig{CookieBlockKey: "myblockkey"}
	key := wc.GetBlockKey()
	if string(key) != "myblockkey" {
		t.Errorf("expected block key 'myblockkey', got %s", string(key))
	}
}

func TestGetConfigFilePath_Default(t *testing.T) {
	os.Unsetenv("CONFIG_PATH")
	os.Unsetenv("CONFIG_DIR")
	expected := filepath.Join(getCwd(t), "config.yaml")
	got := getConfigFilePath()
	if got != expected {
		t.Errorf("expected config path %s, got %s", expected, got)
	}
}

func TestGetConfigFilePath_WithEnv(t *testing.T) {
	os.Setenv("CONFIG_PATH", "/tmp/myconfig.yaml")
	defer os.Unsetenv("CONFIG_PATH")
	got := getConfigFilePath()
	if got != "/tmp/myconfig.yaml" {
		t.Errorf("expected /tmp/myconfig.yaml, got %s", got)
	}
}

func TestGetConfigFromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	content := `
web:
  base_url: "http://test:1234"
  port: 1234
log:
  level: "info"
dashboard:
  reload:
    poll_interval: 2s
`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	tmpFile.Close()
	os.Setenv("CONFIG_PATH", tmpFile.Name())
	defer os.Unsetenv("CONFIG_PATH")

	cfg := GetConfigFromFile()
	if cfg.WebConfig.Port != 1234 {
		t.Errorf("expected port 1234, got %d", cfg.WebConfig.Port)
	}
	if cfg.WebConfig.BaseURL != "http://test:1234" {
		t.Errorf("expected base_url http://test:1234, got %s", cfg.WebConfig.BaseURL)
	}
	if cfg.LogConfig.LevelStr != "info" {
		t.Errorf("expected log level info, got %s", cfg.LogConfig.LevelStr)
	}
	if cfg.Dashboard.Reload.PollInterval != 2*time.Second {
		t.Errorf("expected poll interval 2s, got %v", cfg.Dashboard.Reload.PollInterval)
	}
}

func TestGetConfig_EnvOverride(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	content := `
web:
  base_url: "http://test:1234"
  port: 1234
log:
  level: "info"
dashboard:
  reload:
    poll_interval: 2s
`
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	tmpFile.Close()
	os.Setenv("CONFIG_PATH", tmpFile.Name())
	defer os.Unsetenv("CONFIG_PATH")
	os.Setenv("SDS__WEB_PORT", "4321")
	defer os.Unsetenv("SDS__WEB_PORT")
	os.Setenv("SDS__LOG_LEVEL", "error")
	defer os.Unsetenv("SDS__LOG_LEVEL")

	// Reset package global config
	config = nil

	cfg := GetConfig()
	if cfg.WebConfig.Port != 4321 {
		t.Errorf("expected port 4321 from env, got %d", cfg.WebConfig.Port)
	}
	if cfg.LogConfig.Level != 2 { // logrus.ErrorLevel == 2
		t.Errorf("expected log level 2 (error), got %d", cfg.LogConfig.Level)
	}
}

func TestProcessOidcConfigs(t *testing.T) {
	os.Setenv("SDS__OIDC_TEST_CLIENT_ID", "env-client")
	defer os.Unsetenv("SDS__OIDC_TEST_CLIENT_ID")
	oidc := map[string]*OidcConfig{
		"test": {ClientID: "file-client"},
	}
	result := processOidcConfigs(oidc)
	if result["test"].ClientID != "env-client" {
		t.Errorf("expected env-client, got %s", result["test"].ClientID)
	}
}

func TestGetConfigFromEnv_Nil(t *testing.T) {
	var c *OidcConfig
	got := getConfigFromEnv(c, "SDS__OIDC_TEST_")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func getCwd(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	return cwd
}
