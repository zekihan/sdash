package proxmoxinfo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zekihan/sdash/config"
)

func TestValidNodeName(t *testing.T) {
	for _, node := range []string{"pve1", "PVE-01", "a", "8", strings.Repeat("a", 63)} {
		if !ValidNodeName(node) {
			t.Errorf("ValidNodeName(%q) = false, want true", node)
		}
	}

	for _, node := range []string{
		"", "-pve", "pve-", "pve.local", "pve_1", "pve 1", "pvé", "pve\n",
		"pve/", "pve\\", "pve%", "pve?", "pve#", strings.Repeat("a", 64),
	} {
		if ValidNodeName(node) {
			t.Errorf("ValidNodeName(%q) = true, want false", node)
		}
	}
}

func TestStartGuestValidatesNode(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		if r.Method != http.MethodPost ||
			r.URL.Path != "/api2/json/nodes/PVE-01/lxc/101/status/start" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := new(config.ProxmoxConfig)
	cfg.URL = server.URL
	client := New(*cfg)

	err := client.StartGuest(t.Context(), "pve_1", "lxc", 101)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("StartGuest with invalid node = %v, want ErrUnavailable", err)
	}

	if got := requests.Load(); got != 0 {
		t.Errorf("StartGuest with invalid node sent %d requests, want 0", got)
	}

	err = client.StartGuest(t.Context(), "PVE-01", "lxc", 101)
	if err != nil {
		t.Fatalf("StartGuest with valid node: %v", err)
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("StartGuest sent %d requests, want 1", got)
	}
}

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

func TestSummaryUsesReportedSharedStorageCapacity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"type":"storage","node":"pve-1","storage":"backup","status":"unknown","shared":1},
			{"type":"storage","node":"pve-2","storage":"backup","status":"available",
			 "disk":50,"maxdisk":200,"shared":1},
			{"type":"storage","node":"pve-3","storage":"backup","status":"available",
			 "disk":50,"maxdisk":200,"shared":1},
			{"type":"storage","node":"pve-1","storage":"local","status":"available",
			 "disk":10,"maxdisk":100,"shared":0},
			{"type":"storage","node":"pve-2","storage":"local","status":"available",
			 "disk":20,"maxdisk":300,"shared":0}
		]}`))
	}))
	defer server.Close()

	cfg := new(config.ProxmoxConfig)
	cfg.URL = server.URL

	summary, err := New(*cfg).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if summary.StorageUsed != 80 || summary.StorageTotal != 600 {
		t.Errorf(
			"storage used/total = %d/%d, want 80/600",
			summary.StorageUsed,
			summary.StorageTotal,
		)
	}

	if len(summary.Storage) != 3 {
		t.Fatalf("storage = %+v, want one shared row and two local rows", summary.Storage)
	}

	shared := summary.Storage[0]
	if shared.Name != "backup" || shared.Used != 50 || shared.Total != 200 || shared.UsedPct != 25 {
		t.Errorf("shared storage = %+v, want backup with 50/200 used (25%%)", shared)
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
