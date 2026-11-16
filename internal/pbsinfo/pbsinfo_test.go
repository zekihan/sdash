package pbsinfo

import (
	"net/http"
	"net/http/httptest"
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
}
