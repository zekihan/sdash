package proxmoxinfo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=root@pam!sdash=secret" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = w.Write([]byte(`{"data":[
			{"type":"node","node":"pve-1","status":"online","cpu":0.25,"mem":400,"maxmem":1600},
			{"type":"node","node":"pve-2","status":"offline","cpu":0,"mem":0,"maxmem":1600},
			{"type":"qemu","node":"pve-1","name":"vm-web","status":"running","vmid":100},
			{"type":"lxc","node":"pve-1","name":"ct-dns","status":"stopped","vmid":101},
			{"type":"storage","node":"pve-1","storage":"shared","status":"available","disk":50,"maxdisk":200,"shared":1},
			{"type":"storage","node":"pve-2","storage":"shared","status":"available","disk":50,"maxdisk":200,"shared":1},
			{"type":"storage","node":"pve-1","storage":"local","status":"available","disk":10,"maxdisk":100,"shared":0}
		]}`))
	}))
	defer server.Close()

	cfg := new(config.ProxmoxConfig)
	cfg.URL = server.URL
	cfg.APIToken = "root@pam!sdash=secret"

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.NodesOnline != 1 || summary.NodesTotal != 2 {
		t.Errorf("nodes = %d/%d, want 1/2", summary.NodesOnline, summary.NodesTotal)
	}

	if summary.Nodes[0].CPUPercent != 25 || summary.Nodes[0].MemPercent != 25 {
		t.Errorf(
			"node percents = %v/%v, want 25/25",
			summary.Nodes[0].CPUPercent,
			summary.Nodes[0].MemPercent,
		)
	}

	if summary.GuestsRun != 1 || summary.GuestsTotal != 2 || len(summary.Stopped) != 1 {
		t.Errorf("guests run=%d total=%d stopped=%d, want 1/2/1",
			summary.GuestsRun, summary.GuestsTotal, len(summary.Stopped))
	}

	if len(summary.Storage) != 2 || summary.StorageTotal != 300 {
		t.Errorf(
			"storage = %+v total=%d, want shared counted once (300)",
			summary.Storage,
			summary.StorageTotal,
		)
	}
}

func TestSummaryExcludesGuestTemplates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"type":"qemu","node":"pve-1","name":"vm-web","status":"running","vmid":100},
			{"type":"lxc","node":"pve-1","name":"ct-dns","status":"stopped","vmid":101,"template":0},
			{"type":"qemu","node":"pve-1","name":"vm-base","status":"stopped","vmid":102,"template":1},
			{"type":"lxc","node":"pve-1","name":"ct-base","status":"stopped","vmid":103,"template":1}
		]}`))
	}))
	defer server.Close()

	cfg := new(config.ProxmoxConfig)
	cfg.URL = server.URL

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.GuestsRun != 1 || summary.GuestsTotal != 2 {
		t.Errorf("guests running/total = %d/%d, want 1/2", summary.GuestsRun, summary.GuestsTotal)
	}

	if len(summary.Stopped) != 1 || summary.Stopped[0].VMID != 101 {
		t.Errorf("stopped guests = %+v, want only guest 101", summary.Stopped)
	}
}

func TestSummaryUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := new(config.ProxmoxConfig)
	cfg.URL = server.URL
	cfg.APIToken = "bad"

	_, err := New(*cfg).Summary(t.Context())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want 401 unavailable", err)
	}
}
