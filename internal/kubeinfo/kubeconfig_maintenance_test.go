package kubeinfo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestKubeconfigClientAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	fixture := `current-context: selected
clusters:
- name: unused
  cluster: {server: "https://unused"}
- name: cluster
  cluster: {server: "http://127.0.0.1:1234/", insecure-skip-tls-verify: true}
contexts:
- name: selected
  context: {cluster: cluster, user: user}
users:
- name: user
  user: {token: fixture-token}
- name: unused
  user: {token: wrong}
`

	err := os.WriteFile(path, []byte(fixture), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg := new(config.KubernetesConfig)
	cfg.Kubeconfig = path

	client, err := New(*cfg)
	if err != nil {
		t.Fatal(err)
	}

	if client.base != "http://127.0.0.1:1234" || client.token != "fixture-token" ||
		client.argocdNS != "argocd" {
		t.Fatalf("unexpected client: %+v", client)
	}

	for _, tc := range []struct {
		want    error
		name    string
		content string
	}{
		{want: ErrBadConfig, name: "invalid yaml", content: "["},
		{want: ErrBadConfig, name: "missing server", content: "current-context: missing\n"},
		{
			want: ErrBadConfig, name: "exec plugin",
			content: "clusters:\n- cluster: {server: 'https://localhost'}\nusers:\n- user:\n    exec: {command: forbidden}\n",
		},
		{
			want: ErrBadConfig, name: "bad ca",
			content: "clusters:\n- cluster: {server: 'https://localhost', certificate-authority-data: '!!!'}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeErr := os.WriteFile(path, []byte(tc.content), 0o600)
			if writeErr != nil {
				t.Fatal(writeErr)
			}

			_, configErr := New(*cfg)
			if !errors.Is(configErr, tc.want) {
				t.Fatalf("error = %v, want %v", configErr, tc.want)
			}
		})
	}

	_, err = loadKubeconfig(filepath.Join(dir, "missing"))
	if !errors.Is(err, ErrNoConfig) {
		t.Fatalf("missing config error = %v", err)
	}
}
