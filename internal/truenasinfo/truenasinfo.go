// Package truenasinfo reads a TrueNAS system summary (version, pools,
// alerts) from the REST v2 API using a read-only API key.
package truenasinfo

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
	maxResponseBytes = 4 << 20
	percentScale     = 100.0
)

var (
	ErrUnavailable = errors.New("truenas API unavailable")
	ErrBadResponse = errors.New("invalid truenas API response")
)

// Pool is one ZFS pool's state.
type Pool struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	UsedPct float64 `json:"usedPercent"`
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Healthy bool    `json:"healthy"`
}

// Summary is the dashboard-facing TrueNAS overview.
type Summary struct {
	Hostname          string `json:"hostname"`
	Version           string `json:"version"`
	Pools             []Pool `json:"pools"`
	PoolsOK           int    `json:"poolsHealthy"`
	PoolsTotal        int    `json:"poolsTotal"`
	Alerts            int    `json:"alerts"`
	Used              uint64 `json:"used"`
	Total             uint64 `json:"total"`
	Available         bool   `json:"available"`
	AlertsUnavailable bool   `json:"alertsUnavailable,omitempty"`
}

type apiSystemInfo struct {
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
}

type apiPool struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Healthy bool   `json:"healthy"`
	Size    uint64 `json:"size"`
	Alloc   uint64 `json:"allocated"`
}

type apiAlert struct {
	Dismissed bool `json:"dismissed"`
}

// Client reads from one TrueNAS host.
type Client struct {
	httpClient *http.Client
	base       string
	apiKey     string
}

// New builds a client from the truenas config.
func New(cfg config.TrueNASConfig) *Client {
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
	client.apiKey = cfg.APIKey

	return client
}

// Summary fetches system info, pools and active alerts.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	summary := new(Summary)
	summary.Pools = []Pool{}

	info := new(apiSystemInfo)

	err := client.getJSON(ctx, "/api/v2.0/system/info", info)
	if err != nil {
		return nil, err
	}

	summary.Hostname = info.Hostname
	summary.Version = info.Version

	var pools []apiPool

	err = client.getJSON(ctx, "/api/v2.0/pool", &pools)
	if err != nil {
		return nil, err
	}

	for _, apiPoolValue := range pools {
		pool := new(Pool)
		pool.Name = apiPoolValue.Name
		pool.Status = apiPoolValue.Status
		pool.Healthy = apiPoolValue.Healthy
		pool.Used = apiPoolValue.Alloc
		pool.Total = apiPoolValue.Size

		if apiPoolValue.Size > 0 {
			pool.UsedPct = float64(apiPoolValue.Alloc) / float64(apiPoolValue.Size) * percentScale
		}

		summary.PoolsTotal++
		summary.Used += apiPoolValue.Alloc
		summary.Total += apiPoolValue.Size

		if apiPoolValue.Healthy {
			summary.PoolsOK++
		}

		summary.Pools = append(summary.Pools, *pool)
	}

	sort.Slice(summary.Pools, func(first, second int) bool {
		return summary.Pools[first].Name < summary.Pools[second].Name
	})

	var alerts []apiAlert

	err = client.getJSON(ctx, "/api/v2.0/alert/list", &alerts)
	if err != nil {
		summary.AlertsUnavailable = true
	} else {
		for _, alert := range alerts {
			if !alert.Dismissed {
				summary.Alerts++
			}
		}
	}

	summary.Available = true

	return summary, nil
}

func (client *Client) getJSON(ctx context.Context, apiPath string, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, client.base+apiPath, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)

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
