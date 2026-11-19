package gatusinfo

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"web","group":"core","results":[
				{"duration":3000000,"success":true},
				{"duration":5000000,"success":true}]},
			{"name":"db","group":"core","results":[
				{"duration":9000000,"success":true},
				{"duration":11000000,"success":false}]},
			{"name":"empty","group":"misc","results":[]}
		]`))
	}))
	defer server.Close()

	summary, err := New(server.URL).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.Total != 3 || summary.Up != 1 || summary.Down != 2 {
		t.Fatalf("totals = %+v, want 3 total, 1 up, 2 down", summary)
	}

	// Down endpoints sort first.
	if summary.Endpoints[0].Up {
		t.Errorf("first endpoint = %+v, want a down one first", summary.Endpoints[0])
	}

	for _, endpoint := range summary.Endpoints {
		if endpoint.Name == "web" {
			if endpoint.LatencyMs != 5 || endpoint.Uptime != 100 {
				t.Errorf("web = %+v, want 5ms at 100%%", endpoint)
			}
		}

		if endpoint.Name == "db" && endpoint.Uptime != 50 {
			t.Errorf("db uptime = %v, want 50", endpoint.Uptime)
		}
	}
}

func TestSummaryResponseLimit(t *testing.T) {
	const statuses = `[]`

	padded := statuses + strings.Repeat(" ", maxResponseBytes-len(statuses))

	for _, test := range []struct {
		want error
		name string
		body string
	}{
		{name: "at limit", body: padded, want: nil},
		{name: "over limit", body: padded + " ", want: ErrBadResponse},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = io.WriteString(w, test.body)
				}),
			)
			defer server.Close()

			summary, err := New(server.URL).Summary(t.Context())
			if !errors.Is(err, test.want) {
				t.Fatalf("Summary = %+v, %v, want error %v", summary, err, test.want)
			}

			if test.want == nil && (summary == nil || !summary.Available) {
				t.Fatalf("Summary = %+v, want an available summary", summary)
			}
		})
	}
}
