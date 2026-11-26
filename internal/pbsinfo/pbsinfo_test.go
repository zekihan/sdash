package pbsinfo

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestSummary(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/api2/json/status/datastore-usage",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "PBSAPIToken=root@pam!sdash:secret" {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			_, _ = w.Write([]byte(`{"data":[
			{"store":"backups","total":1000,"used":400},
			{"store":"iso","total":100,"used":10}
		]}`))
		},
	)
	mux.HandleFunc(
		"/api2/json/nodes/localhost/tasks",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":[
			{"type":"backup","starttime":500},
			{"status":"OK","type":"backup","starttime":100,"endtime":200},
			{"status":"backup failed","type":"backup","starttime":300,"endtime":400}
		]}`))
		},
	)

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := new(config.PBSConfig)
	cfg.URL = server.URL
	cfg.APIToken = "root@pam!sdash:secret"
	cfg.Node = "localhost"

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if len(summary.Datastores) != 2 || summary.Total != 1100 || summary.Used != 410 {
		t.Errorf("datastores = %+v, want 2 stores 410/1100", summary)
	}

	if summary.RecentTotal != 2 || summary.RecentFailures != 1 || summary.LastBackupOK {
		t.Errorf("backups = total %d failures %d lastOK %v, want 2/1/false",
			summary.RecentTotal, summary.RecentFailures, summary.LastBackupOK)
	}

	if !summary.HasBackups || summary.LastBackupAt != 400 {
		t.Errorf("last backup = %d, has backups = %v, want a completed backup at 400",
			summary.LastBackupAt, summary.HasBackups)
	}

	if summary.TasksUnavailable {
		t.Error("readable backup task list was marked unavailable")
	}
}

func TestSummaryTasksUnavailable(t *testing.T) {
	token := "fixture-" + t.Name()

	var logs bytes.Buffer

	originalLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/api2/json/status/datastore-usage",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"store":"backups","total":1000,"used":400}]}`))
		},
	)
	mux.HandleFunc(
		"/api2/json/nodes/localhost/tasks",
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, token, http.StatusForbidden)
		},
	)

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := new(config.PBSConfig)
	cfg.URL = server.URL
	cfg.APIToken = token

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if !summary.TasksUnavailable || !summary.Available || len(summary.Datastores) != 1 {
		t.Fatalf(
			"summary = %+v, want available datastore usage and unavailable backup tasks",
			summary,
		)
	}

	if summary.Used != 400 || summary.Total != 1000 {
		t.Errorf("datastore usage = %d/%d, want 400/1000", summary.Used, summary.Total)
	}

	output := logs.String()
	if !strings.Contains(output, "level=WARN") ||
		!strings.Contains(output, "PBS backup task fetch failed") {
		t.Error("expected a warning for the backup task fetch failure")
	}

	if strings.Contains(output, token) {
		t.Error("warning contains the PBS token")
	}
}
