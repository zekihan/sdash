// Package crowdsecinfo reads active decision counts from a CrowdSec local
// API using a bouncer API key.
package crowdsecinfo

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

	"github.com/zekihan/sdash/config"
)

const (
	defaultTimeout   = 6 * time.Second
	maxResponseBytes = 16 << 20
	maxListed        = 10
)

var (
	ErrUnavailable = errors.New("crowdsec API unavailable")
	ErrBadResponse = errors.New("invalid crowdsec API response")
)

// DecisionGroup aggregates active decisions sharing origin and type.
type DecisionGroup struct {
	Origin string `json:"origin"`
	Type   string `json:"type"`
	Count  int    `json:"count"`
}

// Summary is the dashboard-facing CrowdSec overview.
type Summary struct {
	Groups    []DecisionGroup `json:"groups"`
	Decisions int             `json:"decisions"`
	Available bool            `json:"available"`
}

type apiDecision struct {
	Origin string `json:"origin"`
	Type   string `json:"type"`
}

// Client reads from one CrowdSec LAPI.
type Client struct {
	httpClient *http.Client
	base       string
	apiKey     string
}

// New builds a client from the crowdsec config.
func New(cfg config.CrowdSecConfig) *Client {
	httpClient := new(http.Client)
	httpClient.Timeout = defaultTimeout

	client := new(Client)
	client.httpClient = httpClient
	client.base = strings.TrimSuffix(cfg.URL, "/")
	client.apiKey = cfg.APIKey

	return client
}

// Summary counts active decisions grouped by origin and type.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodGet,
		client.base+"/v1/decisions",
		http.NoBody,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", client.apiKey)

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

	var decisions []apiDecision

	// The API returns the literal "null" when nothing is banned.
	if strings.TrimSpace(string(content)) != "null" {
		err = json.Unmarshal(content, &decisions)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadResponse, err)
		}
	}

	summary := new(Summary)
	summary.Available = true
	summary.Decisions = len(decisions)
	summary.Groups = []DecisionGroup{}

	counts := make(map[string]*DecisionGroup)

	for _, decision := range decisions {
		key := decision.Origin + "|" + decision.Type

		group, ok := counts[key]
		if !ok {
			group = new(DecisionGroup)
			group.Origin = decision.Origin
			group.Type = decision.Type
			counts[key] = group
		}

		group.Count++
	}

	for _, group := range counts {
		summary.Groups = append(summary.Groups, *group)
	}

	sort.Slice(summary.Groups, func(first, second int) bool {
		return summary.Groups[first].Count > summary.Groups[second].Count
	})

	if len(summary.Groups) > maxListed {
		summary.Groups = summary.Groups[:maxListed]
	}

	return summary, nil
}
