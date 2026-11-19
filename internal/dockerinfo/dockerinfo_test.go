package dockerinfo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContainersOverTCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1.44/containers/json" ||
			r.URL.Query().Get("all") != "true" {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write([]byte(`[{"Id":"fixture-id","Names":["/example"],"State":"running"}]`))
	}))
	defer server.Close()

	client, err := New("tcp://" + strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	containers, err := client.Containers(t.Context())
	if err != nil {
		t.Fatalf("Containers: %v", err)
	}

	if len(containers) != 1 || containers[0].Name != "example" || containers[0].State != "running" {
		t.Fatalf("Containers = %+v, want one running container named example", containers)
	}
}
