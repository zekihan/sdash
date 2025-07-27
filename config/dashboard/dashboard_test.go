package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

// mockConfigFile creates a temporary YAML config file for testing.
func mockConfigFile(t *testing.T, content string) (string, func()) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dashboard.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mock config file: %v", err)
	}
	origEnv := os.Getenv("DASHBOARD_CONFIG_PATH")
	os.Setenv("DASHBOARD_CONFIG_PATH", filePath)
	cleanup := func() {
		os.Remove(filePath)
		os.Setenv("DASHBOARD_CONFIG_PATH", origEnv)
	}
	return filePath, cleanup
}

func TestGetConfigFromFile_Success(t *testing.T) {
	yamlContent := `
services:
  testsvc:
    name: "Test Service"
groups:
  testgrp:
    name: "Test Group"
`
	_, cleanup := mockConfigFile(t, yamlContent)
	defer cleanup()

	cfg, err := getConfigFromFile()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Services["testsvc"] == nil {
		t.Errorf("expected service 'testsvc' to be present")
	}
	if cfg.Groups["testgrp"] == nil {
		t.Errorf("expected group 'testgrp' to be present")
	}
}

func TestGetConfigFromFile_IsDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DASHBOARD_CONFIG_PATH", tmpDir)
	defer os.Unsetenv("DASHBOARD_CONFIG_PATH")
	_, err := getConfigFromFile()
	if err == nil || err.Error() != "config file is a directory" {
		t.Errorf("expected directory error, got %v", err)
	}
}

func TestGetConfigFilePath_EnvVar(t *testing.T) {
	os.Setenv("DASHBOARD_CONFIG_PATH", "/tmp/test-dashboard.yaml")
	defer os.Unsetenv("DASHBOARD_CONFIG_PATH")
	got := getConfigFilePath()
	if got != "/tmp/test-dashboard.yaml" {
		t.Errorf("expected /tmp/test-dashboard.yaml, got %s", got)
	}
}

func TestGetConfigFilePath_Default(t *testing.T) {
	os.Unsetenv("DASHBOARD_CONFIG_PATH")
	os.Unsetenv("DASHBOARD_CONFIG_DIR")
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, configFile)
	got := getConfigFilePath()
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestReload_ConfigSuccess(t *testing.T) {
	yamlContent := `
services:
  testsvc:
    name: "Test Service"
`
	_, cleanup := mockConfigFile(t, yamlContent)
	defer cleanup()
	// Set config to nil to force reload
	config = nil
	reload()
	if config == nil || config.Services["testsvc"] == nil {
		t.Errorf("expected config to be loaded with 'testsvc'")
	}
}

func TestGetCurrentConfig_ThreadSafe(t *testing.T) {
	// Just ensure it returns the global config pointer
	config = &Config{}
	got := getCurrentConfig()
	if got != config {
		t.Errorf("expected to get global config pointer")
	}
}

func TestPostProcessConfig_CallsAll(t *testing.T) {
	c := &Config{}
	c.postProcessConfig()
	// No panic = success, as postProcess* are not implemented here
}
