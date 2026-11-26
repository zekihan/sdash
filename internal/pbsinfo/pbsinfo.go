// Package pbsinfo reads a Proxmox Backup Server summary (datastore usage
// and recent backup task outcomes) using a read-only API token.
package pbsinfo

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/zekihan/sdash/config"
)

const (
	defaultTimeout   = 6 * time.Second
	maxResponseBytes = 8 << 20
	taskFetchLimit   = 50
	percentScale     = 100.0
)

var (
	ErrUnavailable = errors.New("proxmox backup server API unavailable")
	ErrBadResponse = errors.New("invalid proxmox backup server API response")
)

// Datastore is one backup datastore's usage.
type Datastore struct {
	Name    string  `json:"name"`
	UsedPct float64 `json:"usedPercent"`
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
}

// Summary is the dashboard-facing PBS overview.
type Summary struct {
	Datastores       []Datastore `json:"datastores"`
	Used             uint64      `json:"used"`
	Total            uint64      `json:"total"`
	LastBackupAt     int64       `json:"lastBackupAt"`
	RecentTotal      int         `json:"recentTotal"`
	RecentFailures   int         `json:"recentFailures"`
	LastBackupOK     bool        `json:"lastBackupOk"`
	HasBackups       bool        `json:"hasBackups"`
	TasksUnavailable bool        `json:"tasksUnavailable"`
	Available        bool        `json:"available"`
}

type apiDatastoreUsage struct {
	Store string `json:"store"`
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

type apiTask struct {
	Status    string `json:"status"`
	Type      string `json:"type"`
	StartTime int64  `json:"starttime"`
	EndTime   int64  `json:"endtime"`
}

type apiList[T any] struct {
	Data []T `json:"data"`
}

// Client reads from one Proxmox Backup Server.
type Client struct {
	httpClient *http.Client
	base       string
	token      string
	node       string
}

// New builds a client from the PBS config.
func New(cfg config.PBSConfig) *Client {
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
	client.node = cfg.Node

	if client.node == "" {
		client.node = "localhost"
	}

	return client
}

// Summary fetches datastore usage and recent backup task outcomes.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	usage := new(apiList[apiDatastoreUsage])

	err := client.getJSON(ctx, "/api2/json/status/datastore-usage", usage)
	if err != nil {
		return nil, err
	}

	summary := new(Summary)
	summary.Available = true
	summary.Datastores = []Datastore{}

	for _, entry := range usage.Data {
		datastore := new(Datastore)
		datastore.Name = entry.Store
		datastore.Used = entry.Used
		datastore.Total = entry.Total

		if entry.Total > 0 {
			datastore.UsedPct = float64(entry.Used) / float64(entry.Total) * percentScale
		}

		summary.Used += entry.Used
		summary.Total += entry.Total
		summary.Datastores = append(summary.Datastores, *datastore)
	}

	sort.Slice(summary.Datastores, func(first, second int) bool {
		return summary.Datastores[first].Name < summary.Datastores[second].Name
	})

	err = client.addBackupTasks(ctx, summary)
	if err != nil {
		summary.TasksUnavailable = true

		slog.WarnContext(ctx, "PBS backup task fetch failed")
	}

	return summary, nil
}

func (client *Client) addBackupTasks(ctx context.Context, summary *Summary) error {
	tasks := new(apiList[apiTask])

	path := fmt.Sprintf(
		"/api2/json/nodes/%s/tasks?limit=%d&typefilter=backup",
		client.node,
		taskFetchLimit,
	)

	err := client.getJSON(ctx, path, tasks)
	if err != nil {
		return err
	}

	for _, task := range tasks.Data {
		// PBS omits endtime for running tasks.
		if task.EndTime == 0 {
			continue
		}

		summary.RecentTotal++

		if task.Status != "OK" {
			summary.RecentFailures++
		}

		if task.EndTime > summary.LastBackupAt {
			summary.LastBackupAt = task.EndTime
			summary.LastBackupOK = task.Status == "OK"
			summary.HasBackups = true
		}
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
	req.Header.Set("Authorization", "PBSAPIToken="+client.token)

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
