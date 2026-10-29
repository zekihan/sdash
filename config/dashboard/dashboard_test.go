package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

// mockConfigFile creates a temporary YAML config file for testing.
func mockConfigFile(t *testing.T, content string) {
	t.Helper()
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "dashboard.yaml")

	err := os.WriteFile(filePath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("failed to write mock config file: %v", err)
	}

	t.Setenv("DASHBOARD_CONFIG_PATH", filePath)
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

	mockConfigFile(t, yamlContent)

	cfg, err := getConfigFromFile(t.Context())
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

	t.Setenv("DASHBOARD_CONFIG_PATH", tmpDir)

	_, err := getConfigFromFile(t.Context())
	if err == nil || err.Error() != "config file is a directory" {
		t.Errorf("expected directory error, got %v", err)
	}
}

func TestGetConfigFilePath_EnvVar(t *testing.T) {
	t.Setenv("DASHBOARD_CONFIG_PATH", "/tmp/test-dashboard.yaml")

	got := getConfigFilePath(t.Context())
	if got != "/tmp/test-dashboard.yaml" {
		t.Errorf("expected /tmp/test-dashboard.yaml, got %s", got)
	}
}

func TestGetConfigFilePath_Default(t *testing.T) {
	os.Unsetenv("DASHBOARD_CONFIG_PATH")
	os.Unsetenv("DASHBOARD_CONFIG_DIR")

	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, configFile)

	got := getConfigFilePath(t.Context())
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

	mockConfigFile(t, yamlContent)
	// Set config to nil to force reload
	config = nil

	reload(t.Context())

	if config == nil || config.Services["testsvc"] == nil {
		t.Errorf("expected config to be loaded with 'testsvc'")
	}
}

func TestGetCurrentConfig_ThreadSafe(t *testing.T) {
	// Just ensure it returns the global config pointer
	config = new(Config)

	got := getCurrentConfig(t.Context())
	if got != config {
		t.Errorf("expected to get global config pointer")
	}
}

func TestPostProcessConfig_CallsAll(t *testing.T) {
	c := new(Config)
	c.postProcessConfig()
	// No panic = success, as postProcess* are not implemented here
}

func TestLoadedGroupsIncludeGeneratedGroupsInEveryOrdering(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ordering string
		want     []string
	}{
		{"none", "", nil},
		{"key", "key", []string{"custom", "named", "others"}},
		{"title", "title", []string{"named", "others", "custom"}},
		{"order", "order", []string{"others", "named", "custom"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockConfigFile(t, `
orderGroupsVia: `+tc.ordering+`
services:
  first: {title: First, group: custom}
  second: {title: Second, group: custom}
  explicit: {title: Explicit, group: named}
  default: {title: Default}
groups:
  named: {title: Alpha, order: 4}
`)

			cfg, err := getConfigFromFile(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(cfg.Groups) != 3 || len(cfg.GroupList) != 3 {
				t.Fatalf("groups: %d map, %d list", len(cfg.Groups), len(cfg.GroupList))
			}

			checkLoadedGroups(t, cfg, tc.want)
		})
	}
}

func checkLoadedGroups(t *testing.T, cfg *Config, want []string) {
	t.Helper()

	seen := make(map[string]bool)
	for i, group := range cfg.GroupList {
		if seen[group.Key] || cfg.Groups[group.Key] != group || !group.HasAny {
			t.Errorf("invalid group in list: %+v", group)
		}

		seen[group.Key] = true
		if i < len(want) && group.Key != want[i] {
			t.Errorf("group %d = %s, want %s", i, group.Key, want[i])
		}
	}

	if cfg.Groups["named"].Title != "Alpha" || cfg.Groups["named"].Order != 4 {
		t.Error("explicit metadata changed")
	}

	if cfg.Groups[DefaultGroup].Title != "Others" ||
		cfg.Services["default"].Group != DefaultGroup {
		t.Error("default group semantics changed")
	}

	if cfg.Services["first"].Group != "custom" || cfg.Services["second"].Group != "custom" {
		t.Error("generated group membership changed")
	}
}
