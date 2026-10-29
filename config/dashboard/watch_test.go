package dashboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	generalConfig "github.com/zekihan/sdash/config"
)

func prepareReloadTest(t *testing.T) string {
	t.Helper()
	mockConfigFile(t, "services: {initial: {}}")
	previous := getCurrentConfig(t.Context())
	settings := &generalConfig.GetConfigWithContext(t.Context()).Dashboard.Reload
	oldSettings := *settings
	settings.ContinueOnError = true

	t.Cleanup(func() {
		*settings = oldSettings

		mutex.Lock()
		config = previous
		mutex.Unlock()
	})
	reload(t.Context())

	return getConfigFilePath(t.Context())
}

func writeDashboard(t *testing.T, filename, key string) {
	t.Helper()

	err := os.WriteFile(filename, []byte("services: {"+key+": {}}"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func requireService(t *testing.T, key string) {
	t.Helper()

	cfg := getCurrentConfig(t.Context())
	if cfg == nil || cfg.Services[key] == nil {
		t.Fatalf("expected service %q, got %+v", key, cfg)
	}
}

func TestPollingReloadsFullResolutionAndOlderReplacements(t *testing.T) {
	filename := prepareReloadTest(t)
	base := time.Now().Truncate(time.Second).Add(-time.Hour)

	for index, key := range []string{"first", "second", "older", "replacement"} {
		target := filename
		if index >= 2 {
			target = filepath.Join(filepath.Dir(filename), "replacement.yaml")
		}

		writeDashboard(t, target, key)

		stamp := base.Add(time.Duration(index+1) * 100 * time.Millisecond)
		if index >= 2 {
			stamp = stamp.Add(-time.Hour)
		}

		err := os.Chtimes(target, stamp, stamp)
		if err != nil {
			t.Fatal(err)
		}

		if target != filename {
			err := os.Rename(target, filename)
			if err != nil {
				t.Fatal(err)
			}
		}

		reloadIfModTimeChanged(t.Context())
		requireService(t, key)
	}
	// No change should not publish a new configuration.
	previous := getCurrentConfig(t.Context())
	reloadIfModTimeChanged(t.Context())

	if getCurrentConfig(t.Context()) != previous {
		t.Fatal("unchanged timestamp reloaded")
	}
}

func awaitService(t *testing.T, key string, retry func()) {
	t.Helper()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if cfg := getCurrentConfig(t.Context()); cfg != nil && cfg.Services[key] != nil {
			return
		}

		select {
		case <-timeout.C:
			t.Fatalf("timed out waiting for service %q", key)
		case <-ticker.C:
			if retry != nil {
				retry()
			}
		}
	}
}

func TestFsNotifyRepeatedReplacementsAndWrites(t *testing.T) {
	filename := prepareReloadTest(t)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		defer close(done)

		watchConfigFileViaFsNotify(ctx)
	}()

	t.Cleanup(func() {
		cancel()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("watcher did not stop")
		}
	})
	// Retry the first write until the asynchronous watcher has attached.
	awaitService(t, "ready", func() { writeDashboard(t, filename, "ready") })

	for _, key := range []string{"atomic1", "atomic2"} {
		replacement := filepath.Join(filepath.Dir(filename), "replacement.yaml")
		writeDashboard(t, replacement, key)

		err := os.Rename(replacement, filename)
		if err != nil {
			t.Fatal(err)
		}

		awaitService(t, key, nil)
	}

	writeDashboard(t, filename, "ordinary")
	awaitService(t, "ordinary", nil)

	err := os.WriteFile(filename, []byte("services: ["), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// A synchronous reload also verifies invalid YAML cannot replace good data.
	reload(t.Context())
	requireService(t, "ordinary")
	writeDashboard(t, filename, "recovered")
	awaitService(t, "recovered", nil)

	err = os.Remove(filename)
	if err != nil {
		t.Fatal(err)
	}

	writeDashboard(t, filename, "recreated")
	awaitService(t, "recreated", nil)
}

func TestReloadErrorContract(t *testing.T) {
	filename := prepareReloadTest(t)

	err := os.WriteFile(filename, []byte("services: ["), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	reloadIfModTimeChanged(t.Context())
	requireService(t, "initial")
	generalConfig.GetConfigWithContext(t.Context()).Dashboard.Reload.ContinueOnError = false

	defer func() {
		if recover() == nil {
			t.Error("expected panic with ContinueOnError disabled")
		}
	}()

	reload(t.Context())
}

func TestWatchConfigEventsIgnoresOtherFiles(t *testing.T) {
	filename := prepareReloadTest(t)
	previous := getCurrentConfig(t.Context())
	writeDashboard(t, filename, "not-triggered")

	watcher := &fsnotify.Watcher{
		Events: make(chan fsnotify.Event, 2),
		Errors: make(chan error),
	}
	watcher.Events <- fsnotify.Event{Name: filepath.Join(filepath.Dir(filename), "other.yaml"), Op: fsnotify.Write}

	watcher.Events <- fsnotify.Event{Name: filename, Op: fsnotify.Chmod}

	close(watcher.Events)
	watchConfigEvents(t.Context(), watcher, filename)

	if getCurrentConfig(t.Context()) != previous {
		t.Fatal("unrelated event reloaded configuration")
	}
}

func TestReloadPreservesConfigDuringTruncateThenInvalidWrite(t *testing.T) {
	filename := prepareReloadTest(t)

	// Force the intermediate state emitted by O_TRUNC before the writer supplies
	// bytes, rather than relying on the operating system's event timing.
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reload(t.Context())
	requireService(t, "initial")

	_, err = file.WriteString("services: [")
	if err != nil {
		t.Fatal(err)
	}

	reload(t.Context())
	requireService(t, "initial")

	writeDashboard(t, filename, "recovered")
	reload(t.Context())
	requireService(t, "recovered")
}

func TestReloadEmptyDocumentPolicy(t *testing.T) {
	for _, content := range []string{"", "  \n", "# comment\n", "null\n", "---\n"} {
		t.Run(content, func(t *testing.T) {
			filename := prepareReloadTest(t)

			err := os.WriteFile(filename, []byte(content), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			reload(t.Context())
			requireService(t, "initial")
			generalConfig.GetConfigWithContext(t.Context()).Dashboard.Reload.ContinueOnError = false

			defer func() {
				if recover() == nil {
					t.Error("expected panic for an empty document with ContinueOnError disabled")
				}
			}()

			reload(t.Context())
		})
	}
}

func TestReloadExplicitEmptyMapping(t *testing.T) {
	filename := prepareReloadTest(t)

	err := os.WriteFile(filename, []byte("{}\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	reload(t.Context())

	if len(getCurrentConfig(t.Context()).Services) != 0 {
		t.Fatal("explicit empty mapping did not clear services")
	}
}

func TestNullEntryReloadPolicyAndRecovery(t *testing.T) {
	for _, mode := range []string{"polling", "fsnotify"} {
		t.Run(mode, func(t *testing.T) {
			filename := prepareReloadTest(t)
			previous := getCurrentConfig(t.Context())
			trigger := func() {
				if mode == "polling" {
					reloadIfModTimeChanged(t.Context())

					return
				}

				watcher := &fsnotify.Watcher{
					Events: make(chan fsnotify.Event, 1),
					Errors: make(chan error),
				}
				watcher.Events <- fsnotify.Event{Name: filename, Op: fsnotify.Write}

				close(watcher.Events)
				watchConfigEvents(t.Context(), watcher, filename)
			}

			err := os.WriteFile(filename, []byte("groups: {others: null}"), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			stamp := previous.LastModTime.Add(time.Second)

			err = os.Chtimes(filename, stamp, stamp)
			if err != nil {
				t.Fatal(err)
			}

			trigger()

			if getCurrentConfig(t.Context()) != previous {
				t.Fatal("invalid config replaced previous config")
			}

			writeDashboard(t, filename, "recovered")

			stamp = stamp.Add(time.Second)

			err = os.Chtimes(filename, stamp, stamp)
			if err != nil {
				t.Fatal(err)
			}

			trigger()
			requireService(t, "recovered")
		})
	}
}

func TestNullEntryFailsFast(t *testing.T) {
	filename := prepareReloadTest(t)

	err := os.WriteFile(filename, []byte("services: {broken: null}"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	generalConfig.GetConfigWithContext(t.Context()).Dashboard.Reload.ContinueOnError = false

	defer func() {
		if err := recover(); err == nil || !strings.Contains(fmt.Sprint(err), `service "broken"`) {
			t.Errorf("expected contextual panic, got %v", err)
		}
	}()

	reload(t.Context())
}
