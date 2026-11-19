// Package dockerinfo lists containers through the Docker Engine API over a
// unix socket or TCP host. It talks HTTP directly to avoid pulling the full
// Docker SDK for one read-only endpoint; no Docker data leaves the server
// except through the authenticated dashboard API.
package dockerinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultTimeout   = 4 * time.Second
	maxResponseBytes = 4 << 20
	apiVersion       = "v1.44"
	shortIDLength    = 12
)

var (
	ErrUnavailable = errors.New("docker endpoint unavailable")
	ErrBadResponse = errors.New("invalid docker API response")
	ErrNoEndpoint  = errors.New("no docker endpoint found")
)

// apiContainer mirrors the Engine API container summary. Field names are
// dictated by that API.
//
//nolint:tagliatelle // Docker Engine API uses PascalCase keys
type apiContainer struct {
	ID      string    `json:"Id"`
	Image   string    `json:"Image"`
	State   string    `json:"State"`
	Status  string    `json:"Status"`
	Names   []string  `json:"Names"`
	Ports   []apiPort `json:"Ports"`
	Created int64     `json:"Created"`
}

//nolint:tagliatelle // Docker Engine API uses PascalCase keys
type apiPort struct {
	Type        string `json:"Type"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
}

// Port is one published/exposed port mapping in dashboard API responses.
type Port struct {
	Type        string `json:"type"`
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort"`
}

// ContainerJSON is the dashboard-facing container summary.
type ContainerJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Ports   []Port `json:"ports"`
	Created int64  `json:"created"`
}

// Client reads container summaries from one Docker endpoint.
type Client struct {
	httpClient *http.Client
	base       string
}

// New builds a client for host: empty auto-detects the default unix sockets,
// otherwise a socket path, unix:///path or tcp://host:port (or http(s)://).
func New(host string) (*Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = detectSocket()
		if host == "" {
			return nil, ErrNoEndpoint
		}
	}

	client := new(Client)

	if strings.HasPrefix(host, "tcp://") || strings.HasPrefix(host, "http://") ||
		strings.HasPrefix(host, "https://") {
		base := "http://" + strings.TrimPrefix(host, "tcp://")
		if strings.HasPrefix(host, "http") {
			base = host
		}

		client.base = strings.TrimSuffix(base, "/")
		client.httpClient = newHTTPClient(nil)

		return client, nil
	}

	socket := strings.TrimPrefix(host, "unix://")
	transport := new(http.Transport)
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		dialer := new(net.Dialer)

		return dialer.DialContext(ctx, "unix", socket)
	}

	client.base = "http://docker"
	client.httpClient = newHTTPClient(transport)

	return client, nil
}

func newHTTPClient(transport *http.Transport) *http.Client {
	client := new(http.Client)

	client.Timeout = defaultTimeout
	if transport != nil {
		client.Transport = transport
	}

	return client
}

func detectSocket() string {
	for _, candidate := range []string{"/var/run/docker.sock", "/run/docker.sock"} {
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate
		}
	}

	return ""
}

// Containers returns every container (running and stopped) newest first.
func (client *Client) Containers(ctx context.Context) ([]ContainerJSON, error) {
	url := client.base + "/" + apiVersion + "/containers/json?all=true"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	var containers []apiContainer

	err = json.Unmarshal(body, &containers)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	out := make([]ContainerJSON, 0, len(containers))
	for _, container := range containers {
		out = append(out, toContainerJSON(&container))
	}

	return out, nil
}

func toContainerJSON(container *apiContainer) ContainerJSON {
	name := container.ID
	if len(name) > shortIDLength {
		name = name[:shortIDLength]
	}

	if len(container.Names) > 0 {
		name = strings.TrimPrefix(container.Names[0], "/")
	}

	ports := make([]Port, 0, len(container.Ports))
	for _, apiPortValue := range container.Ports {
		port := new(Port)
		port.Type = apiPortValue.Type
		port.PrivatePort = apiPortValue.PrivatePort
		port.PublicPort = apiPortValue.PublicPort
		ports = append(ports, *port)
	}

	out := new(ContainerJSON)
	out.ID = container.ID
	out.Name = name
	out.Image = container.Image
	out.State = container.State
	out.Status = container.Status
	out.Ports = ports
	out.Created = container.Created

	return *out
}
