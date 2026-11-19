package kubeinfo

import (
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestInClusterClientReloadsServiceAccountToken(t *testing.T) {
	var rotated atomic.Bool

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Bearer initial-token"
		if rotated.Load() {
			want = "Bearer rotated-token"
		}

		if got := r.Header.Get("Authorization"); got != want {
			t.Errorf("%s authorization = %q, want %q", r.Method, got, want)
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(server.Close)

	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", host)
	t.Setenv("KUBERNETES_SERVICE_PORT", port)

	accountDir := t.TempDir()
	t.Setenv("KUBECONFIG", filepath.Join(accountDir, "unused-kubeconfig"))

	ca := new(pem.Block)
	ca.Type = "CERTIFICATE"
	ca.Bytes = server.Certificate().Raw
	writeServiceAccountFixture(t, filepath.Join(accountDir, "ca.crt"), pem.EncodeToMemory(ca))
	tokenPath := filepath.Join(accountDir, "token")
	writeServiceAccountFixture(t, tokenPath, []byte("initial-token\n"))

	var cfg config.KubernetesConfig

	client, err := newClient(cfg, accountDir)
	if err != nil {
		t.Fatalf("new in-cluster client: %v", err)
	}

	err = client.getJSON(t.Context(), "/api/v1/nodes", new(nodeList))
	if err != nil {
		t.Fatalf("list nodes before rotation: %v", err)
	}

	writeServiceAccountFixture(t, tokenPath+".next", []byte("rotated-token\n"))

	err = os.Rename(tokenPath+".next", tokenPath)
	if err != nil {
		t.Fatalf("rotate token: %v", err)
	}

	rotated.Store(true)

	err = client.getJSON(t.Context(), "/api/v1/nodes", new(nodeList))
	if err != nil {
		t.Errorf("list nodes after rotation: %v", err)
	}

	err = client.RestartDeployment(t.Context(), "apps", "web")
	if err != nil {
		t.Errorf("restart deployment after rotation: %v", err)
	}
}

func writeServiceAccountFixture(t *testing.T, path string, content []byte) {
	t.Helper()

	err := os.WriteFile(path, content, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
