// Package proxmoxinfo reads a Proxmox VE cluster summary (nodes, guests,
// storage) from one node's API using a read-only API token.
package proxmoxinfo

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/zekihan/sdash/config"
)

const (
	defaultTimeout   = 6 * time.Second
	maxResponseBytes = 8 << 20
	maxListed        = 24
	percentScale     = 100.0
)

var (
	ErrUnavailable = errors.New("proxmox API unavailable")
	ErrBadResponse = errors.New("invalid proxmox API response")
)

// Node is one cluster node's state.
type Node struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemPercent float64 `json:"memPercent"`
	MemUsed    uint64  `json:"memUsed"`
	MemTotal   uint64  `json:"memTotal"`
	Online     bool    `json:"online"`
}

// Storage is one shared storage's usage.
type Storage struct {
	Name    string  `json:"name"`
	UsedPct float64 `json:"usedPercent"`
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
}

// Guest is a VM or container that is not running.
type Guest struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	VMID   int    `json:"vmid"`
}

// Summary is the dashboard-facing Proxmox overview.
type Summary struct {
	Nodes        []Node    `json:"nodes"`
	Storage      []Storage `json:"storage"`
	Stopped      []Guest   `json:"stoppedGuests"`
	WakeHosts    []string  `json:"wakeHosts"`
	NodesOnline  int       `json:"nodesOnline"`
	NodesTotal   int       `json:"nodesTotal"`
	GuestsRun    int       `json:"guestsRunning"`
	GuestsTotal  int       `json:"guestsTotal"`
	StorageUsed  uint64    `json:"storageUsed"`
	StorageTotal uint64    `json:"storageTotal"`
	Available    bool      `json:"available"`
}

type apiResource struct {
	Type     string  `json:"type"`
	Node     string  `json:"node"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Storage  string  `json:"storage"`
	VMID     int     `json:"vmid"`
	CPU      float64 `json:"cpu"`
	Mem      uint64  `json:"mem"`
	MaxMem   uint64  `json:"maxmem"`
	Disk     uint64  `json:"disk"`
	MaxDisk  uint64  `json:"maxdisk"`
	Shared   int     `json:"shared"`
	Template int     `json:"template"`
}

type apiResponse struct {
	Data []apiResource `json:"data"`
}

// Client reads cluster resources from one Proxmox VE node.
type Client struct {
	httpClient *http.Client
	base       string
	token      string
}

// New builds a client from the proxmox config.
func New(cfg config.ProxmoxConfig) *Client {
	tlsConfig := new(tls.Config)
	tlsConfig.MinVersion = tls.VersionTLS12
	tlsConfig.InsecureSkipVerify = cfg.Insecure

	transport := new(http.Transport)
	transport.TLSClientConfig = tlsConfig
	transport.Proxy = http.ProxyFromEnvironment

	httpClient := new(http.Client)
	httpClient.Timeout = defaultTimeout
	httpClient.Transport = transport

	client := new(Client)
	client.httpClient = httpClient
	client.base = strings.TrimSuffix(cfg.URL, "/")
	client.token = cfg.APIToken

	return client
}

// Summary fetches /cluster/resources and aggregates nodes, guests and
// shared storage.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	response := new(apiResponse)

	err := client.getJSON(ctx, "/api2/json/cluster/resources", response)
	if err != nil {
		return nil, err
	}

	summary := new(Summary)
	summary.Available = true
	summary.Nodes = []Node{}
	summary.Storage = []Storage{}
	summary.Stopped = []Guest{}
	summary.WakeHosts = []string{}

	seenStorage := make(map[string]bool)

	for index := range response.Data {
		resource := &response.Data[index]
		switch resource.Type {
		case "node":
			summary.addNode(resource)
		case "qemu", "lxc":
			summary.addGuest(resource)
		case "storage":
			// Shared storage appears once per node; count it once.
			if resource.Shared == 1 && seenStorage[resource.Storage] {
				continue
			}

			seenStorage[resource.Storage] = true
			summary.addStorage(resource)
		}
	}

	sort.Slice(summary.Nodes, func(first, second int) bool {
		return summary.Nodes[first].Name < summary.Nodes[second].Name
	})
	sort.Slice(summary.Storage, func(first, second int) bool {
		return summary.Storage[first].Name < summary.Storage[second].Name
	})
	sort.Slice(summary.Stopped, func(first, second int) bool {
		return summary.Stopped[first].VMID < summary.Stopped[second].VMID
	})

	if len(summary.Stopped) > maxListed {
		summary.Stopped = summary.Stopped[:maxListed]
	}

	return summary, nil
}

func (summary *Summary) addNode(resource *apiResource) {
	node := new(Node)
	node.Name = resource.Node
	node.Online = resource.Status == "online"
	node.CPUPercent = resource.CPU * percentScale
	node.MemUsed = resource.Mem
	node.MemTotal = resource.MaxMem

	if resource.MaxMem > 0 {
		node.MemPercent = float64(resource.Mem) / float64(resource.MaxMem) * percentScale
	}

	summary.NodesTotal++

	if node.Online {
		summary.NodesOnline++
	}

	summary.Nodes = append(summary.Nodes, *node)
}

func (summary *Summary) addGuest(resource *apiResource) {
	if resource.Template == 1 {
		return
	}

	summary.GuestsTotal++

	if resource.Status == "running" {
		summary.GuestsRun++

		return
	}

	guest := new(Guest)
	guest.Name = resource.Name
	guest.Node = resource.Node
	guest.Kind = resource.Type
	guest.Status = resource.Status
	guest.VMID = resource.VMID
	summary.Stopped = append(summary.Stopped, *guest)
}

func (summary *Summary) addStorage(resource *apiResource) {
	storage := new(Storage)
	storage.Name = resource.Storage
	storage.Used = resource.Disk
	storage.Total = resource.MaxDisk

	if resource.MaxDisk > 0 {
		storage.UsedPct = float64(resource.Disk) / float64(resource.MaxDisk) * percentScale
	}

	summary.StorageUsed += resource.Disk
	summary.StorageTotal += resource.MaxDisk
	summary.Storage = append(summary.Storage, *storage)
}

// StartGuest starts a stopped VM (qemu) or container (lxc).
func (client *Client) StartGuest(ctx context.Context, node, kind string, vmid int) error {
	if kind != "qemu" && kind != "lxc" {
		return fmt.Errorf("%w: unsupported guest kind %q", ErrUnavailable, kind)
	}

	path := fmt.Sprintf("/api2/json/nodes/%s/%s/%d/status/start", node, kind, vmid)

	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, client.base+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Authorization", "PVEAPIToken="+client.token)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	_ = copyErr

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: POST %s returned %s", ErrUnavailable, path, resp.Status)
	}

	return nil
}

func (client *Client) getJSON(ctx context.Context, apiPath string, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, client.base+apiPath, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "PVEAPIToken="+client.token)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: GET %s returned %s", ErrUnavailable, apiPath, resp.Status)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	err = json.Unmarshal(content, out)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return nil
}
