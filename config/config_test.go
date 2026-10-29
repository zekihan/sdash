package config

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if cfg.LogConfig.Level != slog.LevelWarn {
		t.Errorf("expected default log level 3, got %d", cfg.LogConfig.Level)
	}

	if cfg.Dashboard.Reload.PollInterval != 5*time.Second {
		t.Errorf("expected default poll interval 5s, got %v", cfg.Dashboard.Reload.PollInterval)
	}
}

func TestWebConfig_GetHashKey_GeneratesKey(t *testing.T) {
	wc := new(WebConfig)

	key := wc.GetHashKey()
	if len(key) != 64 {
		t.Errorf("expected hash key length 64, got %d", len(key))
	}
}

func TestWebConfig_GetHashKey_UsesProvidedKey(t *testing.T) {
	wc := new(WebConfig)
	wc.CookieHashKey = "myhashkey"

	key := wc.GetHashKey()
	if string(key) != "myhashkey" {
		t.Errorf("expected hash key 'myhashkey', got %s", string(key))
	}
}

func TestWebConfig_GetBlockKey_GeneratesKey(t *testing.T) {
	wc := new(WebConfig)

	key := wc.GetBlockKey()
	if len(key) != 32 {
		t.Errorf("expected block key length 32, got %d", len(key))
	}
}

func TestWebConfig_GetBlockKey_UsesProvidedKey(t *testing.T) {
	wc := new(WebConfig)
	wc.CookieBlockKey = "myblockkey"

	key := wc.GetBlockKey()
	if string(key) != "myblockkey" {
		t.Errorf("expected block key 'myblockkey', got %s", string(key))
	}
}

func TestGetConfigFilePath_Default(t *testing.T) {
	os.Unsetenv("CONFIG_PATH")
	os.Unsetenv("CONFIG_DIR")

	expected := filepath.Join(getCwd(t), "config.yaml")

	got := getConfigFilePath(t.Context())
	if got != expected {
		t.Errorf("expected config path %s, got %s", expected, got)
	}
}

func TestGetConfigFilePath_WithEnv(t *testing.T) {
	t.Setenv("CONFIG_PATH", "/tmp/myconfig.yaml")

	got := getConfigFilePath(t.Context())
	if got != "/tmp/myconfig.yaml" {
		t.Errorf("expected /tmp/myconfig.yaml, got %s", got)
	}
}

func TestGetConfigFromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
web:
  baseUrl: "http://test:1234"
  port: 1234
log:
  level: "info"
dashboard:
  reload:
    pollInterval: 2s
`

	_, err = tmpFile.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	t.Setenv("CONFIG_PATH", tmpFile.Name())

	cfg := GetConfigFromFile(t.Context())
	if cfg.WebConfig.Port != 1234 {
		t.Errorf("expected port 1234, got %d", cfg.WebConfig.Port)
	}

	if cfg.WebConfig.BaseURL != "http://test:1234" {
		t.Errorf("expected baseUrl http://test:1234, got %s", cfg.WebConfig.BaseURL)
	}

	if cfg.LogConfig.LevelStr != "info" {
		t.Errorf("expected log level info, got %s", cfg.LogConfig.LevelStr)
	}

	if cfg.Dashboard.Reload.PollInterval != 2*time.Second {
		t.Errorf("expected poll interval 2s, got %v", cfg.Dashboard.Reload.PollInterval)
	}
}

func TestGetConfig_EnvOverride(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
web:
  baseUrl: "http://test:1234"
  port: 1234
log:
  level: "info"
dashboard:
  reload:
    pollInterval: 2s
`

	_, err = tmpFile.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	t.Setenv("CONFIG_PATH", tmpFile.Name())
	t.Setenv("SDS__WEB_PORT", "4321")
	t.Setenv("SDS__WEB_SESSION_TIMEOUT", "2s")
	t.Setenv("SDS__LOG_LEVEL", "error")

	// Reset package global config
	config = nil

	cfg := GetConfigWithContext(t.Context())
	if cfg.WebConfig.SessionTimeout != 2*time.Second {
		t.Errorf("expected timeout 2s from env, got %s", cfg.WebConfig.SessionTimeout)
	}

	if cfg.WebConfig.Port != 4321 {
		t.Errorf("expected port 4321 from env, got %d", cfg.WebConfig.Port)
	}

	if cfg.LogConfig.Level != slog.LevelError {
		t.Errorf("expected log level 2 (error), got %d", cfg.LogConfig.Level)
	}
}

func TestProcessOidcConfigs(t *testing.T) {
	t.Setenv("SDS__OIDC_TEST_CLIENT_ID", "env-client")

	oidc := make(map[string]*OidcConfig)
	oidcConfig := new(OidcConfig)
	oidcConfig.ClientID = "file-client"
	oidc["test"] = oidcConfig

	result := processOidcConfigs(t.Context(), oidc)
	if result["test"].ClientID != "env-client" {
		t.Errorf("expected env-client, got %s", result["test"].ClientID)
	}
}

func TestGetConfigFromEnv_PreservesUnsetAndOverwritesEmpty(t *testing.T) {
	t.Setenv("SDS__OIDC_V2_CLIENT_ID", "")

	oidc := &OidcConfig{
		ClientID:          "file-client",
		ClientSecret:      "file-secret",
		DiscoveryEndpoint: "",
		Enabled:           true,
		Default:           false,
	}

	got := getConfigFromEnv(t.Context(), oidc, "SDS__OIDC_V2_")
	if got.ClientID != "" {
		t.Errorf("expected empty client ID from env, got %q", got.ClientID)
	}

	if got.ClientSecret != "file-secret" {
		t.Errorf("expected unset client secret to be preserved, got %q", got.ClientSecret)
	}

	if !got.Enabled {
		t.Error("expected unset enabled value to be preserved")
	}
}

func TestGetConfigFromEnv_Nil(t *testing.T) {
	var c *OidcConfig

	got := getConfigFromEnv(t.Context(), c, "SDS__OIDC_TEST_")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func getCwd(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	return cwd
}

func TestValidateSessionTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{
		-time.Second, 0, time.Nanosecond, 1500 * time.Millisecond, 31 * 24 * time.Hour,
	} {
		cfg := GetDefaultConfig().WebConfig
		cfg.SessionTimeout = timeout

		err := cfg.ValidateSessionTimeout()
		if err == nil {
			t.Errorf("accepted invalid timeout %s", timeout)
		}
	}

	for _, timeout := range []time.Duration{time.Second, 24 * time.Hour, 30 * 24 * time.Hour} {
		cfg := GetDefaultConfig().WebConfig
		cfg.SessionTimeout = timeout

		err := cfg.ValidateSessionTimeout()
		if err != nil {
			t.Errorf("rejected timeout %s: %v", timeout, err)
		}
	}

	if got := GetDefaultConfig().WebConfig.SessionTimeout; got != 24*time.Hour {
		t.Errorf("default timeout = %s", got)
	}
}

// Exercise the real loader in a child because invalid configuration exits startup.
func TestGetConfigFromFileRequiresMapping(t *testing.T) {
	const childEnv = "SDASH_TEST_CONFIG_MAPPING_CHILD"
	if os.Getenv(childEnv) == "1" {
		GetConfigFromFile(t.Context())

		return
	}

	for _, fixture := range []struct {
		name    string
		content string
		valid   bool
	}{
		{name: "empty", content: "", valid: false},
		{name: "whitespace", content: " \n  ", valid: false},
		{name: "comments", content: "# interrupted config write\n", valid: false},
		{name: "null", content: "null\n", valid: false},
		{name: "tilde", content: "~\n", valid: false},
		{name: "empty document", content: "---\n", valid: false},
		{name: "scalar", content: "disabled\n", valid: false},
		{name: "sequence", content: "[]\n", valid: false},
		{name: "explicit empty mapping", content: "{}\n", valid: true},
		{name: "mapping", content: "web:\n  port: 1234\n", valid: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")

			err := os.WriteFile(configPath, []byte(fixture.content), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("CONFIG_PATH", configPath)
			t.Setenv(childEnv, "1")

			//nolint:gosec // Relaunch only the current test executable, with a fixed test name.
			cmd := exec.CommandContext(t.Context(), os.Args[0],
				"-test.run=^TestGetConfigFromFileRequiresMapping$")

			output, err := cmd.CombinedOutput()
			if fixture.valid {
				if err != nil {
					t.Fatalf("valid config rejected: %v\n%s", err, output)
				}

				return
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("invalid config must exit 1, got %v\n%s", err, output)
			}
		})
	}
}

// Exercise validation after YAML loading and environment overrides, without starting a watcher.
func TestGetConfigRejectsInvalidPollingInterval(t *testing.T) {
	const childEnv = "SDASH_TEST_POLL_INTERVAL_CHILD"
	if os.Getenv(childEnv) == "1" {
		GetConfigWithContext(t.Context())

		return
	}

	for _, fixture := range []struct {
		name     string
		content  string
		override string
		valid    bool
	}{
		{name: "zero", content: "enabled: true, pollInterval: 0s", override: "", valid: false},
		{name: "negative", content: "enabled: true, pollInterval: -1s", override: "", valid: false},
		{name: "env zero", content: "enabled: true, pollInterval: 1s", override: "0s", valid: false},
		{name: "env negative", content: "enabled: true, pollInterval: 1s", override: "-1s", valid: false},
		{name: "env repairs YAML", content: "enabled: true, pollInterval: 0s", override: "1s", valid: true},
		{name: "positive", content: "enabled: true, pollInterval: 1ms", override: "", valid: true},
		{name: "default", content: "enabled: true", override: "", valid: true},
		{name: "disabled", content: "enabled: false, pollInterval: 0s", override: "", valid: true},
		{name: "fsnotify", content: "enabled: true, useFsNotify: true, pollInterval: -1s", override: "", valid: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			content := "dashboard:\n  reload: {" + fixture.content + "}\n"

			err := os.WriteFile(configPath, []byte(content), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("CONFIG_PATH", configPath)
			t.Setenv(childEnv, "1")

			//nolint:gosec // Relaunch only this test executable with a fixed test name.
			cmd := exec.CommandContext(t.Context(), os.Args[0],
				"-test.run=^TestGetConfigRejectsInvalidPollingInterval$")

			for _, variable := range os.Environ() {
				if !strings.HasPrefix(variable, EnvKeyPrefix) {
					cmd.Env = append(cmd.Env, variable)
				}
			}

			if fixture.override != "" {
				cmd.Env = append(cmd.Env, "SDS__DASHBOARD_RELOAD_POLL_INTERVAL="+fixture.override)
			}

			output, err := cmd.CombinedOutput()
			if fixture.valid {
				if err != nil {
					t.Fatalf("valid polling config rejected: %v\n%s", err, output)
				}

				return
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("invalid polling config must exit 1, got %v\n%s", err, output)
			}
		})
	}
}
