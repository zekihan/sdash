package utils

import (
	"github.com/zekihan/sdash/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/snowzach/rotatefilehook"

	log "github.com/sirupsen/logrus"
)

func TestInitLogger_NoFileHook(t *testing.T) {
	// Backup original logger state.
	origLevel := log.GetLevel()
	origFormatter := log.StandardLogger().Formatter
	origOutput := os.Stdout
	origReportCaller := log.StandardLogger().ReportCaller
	origHooks := log.StandardLogger().Hooks

	defer func() {
		log.SetLevel(origLevel)
		log.SetFormatter(origFormatter)
		log.SetReportCaller(origReportCaller)
		log.SetOutput(origOutput)
		log.StandardLogger().Hooks = origHooks
	}()

	cfg := &config.Config{LogConfig: config.LogConfig{
		Level:         log.DebugLevel,
		Path:          "",
		AddMethodName: true,
	}}
	InitLogger(cfg)

	if got := log.GetLevel(); got != log.DebugLevel {
		t.Errorf("level = %d, want %d", got, log.DebugLevel)
	}

	if _, ok := log.StandardLogger().Formatter.(*log.TextFormatter); !ok {
		t.Errorf("formatter = %T, want TextFormatter", log.StandardLogger().Formatter)
	}

	if log.StandardLogger().Out != origOutput {
		t.Errorf("output changed, want %v", origOutput)
	}

	if log.StandardLogger().ReportCaller != cfg.LogConfig.AddMethodName {
		t.Errorf(
			"ReportCaller = %t, want %t",
			log.StandardLogger().ReportCaller,
			cfg.LogConfig.AddMethodName,
		)
	}

	if len(log.StandardLogger().Hooks) != 0 {
		t.Errorf("hooks = %d, want 0", len(log.StandardLogger().Hooks))
	}
}

func TestInitLogger_WithFileHook(t *testing.T) {
	// Backup original logger state.
	origLevel := log.GetLevel()
	origFormatter := log.StandardLogger().Formatter
	origOutput := log.StandardLogger().Out
	origReportCaller := log.StandardLogger().ReportCaller
	origHooks := log.StandardLogger().Hooks

	defer func() {
		log.SetLevel(origLevel)
		log.SetFormatter(origFormatter)
		log.SetReportCaller(origReportCaller)
		log.SetOutput(origOutput)
		log.StandardLogger().Hooks = origHooks
	}()

	tmpFile := filepath.Join(t.TempDir(), "test.log")

	cfg := &config.Config{LogConfig: config.LogConfig{
		Level:         log.WarnLevel,
		Path:          tmpFile,
		AddMethodName: false,
	}}
	InitLogger(cfg)

	if got := log.GetLevel(); got != log.WarnLevel {
		t.Errorf("level = %d, want %d", got, log.WarnLevel)
	}

	hooks := log.StandardLogger().Hooks[log.WarnLevel]
	if len(hooks) == 0 {
		t.Fatal("expected at least one file hook")
	}

	if _, ok := hooks[0].(*rotatefilehook.RotateFileHook); !ok {
		t.Errorf("hook type = %T, want *rotatefilehook.RotateFileHook", hooks[0])
	}
}
