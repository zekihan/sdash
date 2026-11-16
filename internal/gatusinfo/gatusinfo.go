// Package gatusinfo reads endpoint statuses from a Gatus instance
// (https://github.com/TwiN/gatus) and reduces them to up/down counts plus a
// per-endpoint list for the dashboard widget.
package gatusinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout   = 6 * time.Second
	maxResponseBytes = 16 << 20
	maxListed        = 60
	nanosPerMilli    = int64(time.Millisecond)
	percentScale     = 100.0
)

var (
	ErrUnavailable = errors.New("gatus unavailable")
	ErrBadResponse = errors.New("invalid gatus response")
)

// Endpoint is one monitored endpoint's latest state.
type Endpoint struct {
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Uptime    float64 `json:"uptime"`
	LatencyMs int64   `json:"latencyMs"`
	Up        bool    `json:"up"`
}

// Summary is the dashboard-facing Gatus overview.
type Summary struct {
	Endpoints []Endpoint `json:"endpoints"`
	Total     int        `json:"total"`
	Up        int        `json:"up"`
	Down      int        `json:"down"`
	Available bool       `json:"available"`
}

type apiEndpoint struct {
	Name    string `json:"name"`
	Group   string `json:"group"`
	Results []struct {
		Timestamp string `json:"timestamp"`
		Duration  int64  `json:"duration"`
		Success   bool   `json:"success"`
	} `json:"results"`
}

// Client reads statuses from one Gatus base URL.
type Client struct {
	httpClient *http.Client
	base       string
}

// New builds a client for a Gatus base URL (no trailing path).
func New(baseURL string) *Client {
	httpClient := new(http.Client)
	httpClient.Timeout = defaultTimeout

	client := new(Client)
	client.httpClient = httpClient
	client.base = strings.TrimSuffix(baseURL, "/")

	return client
}

// Summary fetches and aggregates the current endpoint statuses.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	content, err := client.fetch(ctx)
	if err != nil {
		return nil, err
	}

	var endpoints []apiEndpoint

	err = json.Unmarshal(content, &endpoints)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	summary := new(Summary)
	summary.Available = true
	summary.Endpoints = []Endpoint{}

	for _, apiEndpointValue := range endpoints {
		endpoint := toEndpoint(&apiEndpointValue)
		summary.Total++

		if endpoint.Up {
			summary.Up++
		} else {
			summary.Down++
		}

		summary.Endpoints = append(summary.Endpoints, endpoint)
	}

	sort.Slice(summary.Endpoints, func(first, second int) bool {
		firstEndpoint, secondEndpoint := summary.Endpoints[first], summary.Endpoints[second]
		if firstEndpoint.Up != secondEndpoint.Up {
			return !firstEndpoint.Up
		}

		if firstEndpoint.Group != secondEndpoint.Group {
			return firstEndpoint.Group < secondEndpoint.Group
		}

		return firstEndpoint.Name < secondEndpoint.Name
	})

	if len(summary.Endpoints) > maxListed {
		summary.Endpoints = summary.Endpoints[:maxListed]
	}

	return summary, nil
}

func toEndpoint(apiEndpointValue *apiEndpoint) Endpoint {
	endpoint := new(Endpoint)
	endpoint.Name = apiEndpointValue.Name
	endpoint.Group = apiEndpointValue.Group

	if len(apiEndpointValue.Results) == 0 {
		return *endpoint
	}

	// Results arrive oldest-first; the last entry is the latest check.
	latest := apiEndpointValue.Results[len(apiEndpointValue.Results)-1]
	endpoint.Up = latest.Success
	endpoint.LatencyMs = latest.Duration / nanosPerMilli

	successful := 0

	for _, result := range apiEndpointValue.Results {
		if result.Success {
			successful++
		}
	}

	endpoint.Uptime = float64(successful) / float64(len(apiEndpointValue.Results)) * percentScale

	return *endpoint
}

func (client *Client) fetch(ctx context.Context) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodGet,
		client.base+"/api/v1/endpoints/statuses",
		http.NoBody,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sdash/1.0")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return content, nil
}
