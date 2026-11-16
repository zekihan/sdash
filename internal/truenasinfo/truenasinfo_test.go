package truenasinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestSummary(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2.0/system/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tn-key" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"hostname":"truenas","version":"TrueNAS-SCALE-24.10"}`))
	})
	mux.HandleFunc("/api/v2.0/pool", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"tank","status":"ONLINE","healthy":true,"size":1000,"allocated":250},
			{"name":"boot-pool","status":"ONLINE","healthy":true,"size":100,"allocated":10}
		]`))
	})
	mux.HandleFunc("/api/v2.0/alert/list", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"dismissed":false},{"dismissed":true},{"dismissed":false}]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := new(config.TrueNASConfig)
	cfg.URL = server.URL
	cfg.APIKey = "tn-key"

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.Hostname != "truenas" || summary.PoolsTotal != 2 || summary.PoolsOK != 2 {
		t.Fatalf("summary = %+v, want truenas with 2 healthy pools", summary)
	}

	if summary.Alerts != 2 {
		t.Errorf("alerts = %d, want 2 undismissed", summary.Alerts)
	}

	if summary.Pools[1].Name != "tank" || summary.Pools[1].UsedPct != 25 {
		t.Errorf("tank pool = %+v, want 25%% used", summary.Pools[1])
	}
}
