package crowdsecinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "cs-key" {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		_, _ = w.Write([]byte(`[
			{"origin":"crowdsec","type":"ban"},
			{"origin":"crowdsec","type":"ban"},
			{"origin":"CAPI","type":"ban"}
		]`))
	}))
	defer server.Close()

	cfg := new(config.CrowdSecConfig)
	cfg.URL = server.URL
	cfg.APIKey = "cs-key"

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.Decisions != 3 || len(summary.Groups) != 2 || summary.Groups[0].Count != 2 {
		t.Errorf("summary = %+v, want 3 decisions in 2 groups, biggest 2", summary)
	}
}

func TestSummaryEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("null"))
	}))
	defer server.Close()

	cfg := new(config.CrowdSecConfig)
	cfg.URL = server.URL

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil || summary.Decisions != 0 {
		t.Fatalf("summary = %+v err=%v, want 0 decisions", summary, err)
	}
}
