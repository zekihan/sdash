// Package appwidgets fetches live per-application data (Sonarr/Radarr
// queues, Jellyfin sessions) for service cards. API keys come from the
// server config and are never sent to browsers; only the normalized stats
// below leave the server.
package appwidgets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

const (
	typeSonarr = "sonarr"
	typeRadarr = "radarr"

	defaultTimeout   = 5 * time.Second
	cacheTTL         = 30 * time.Second
	maxResponseBytes = 2 << 20
	maxLines         = 4
	percentScale     = 100.0
)

var (
	ErrUnavailable = errors.New("application API unavailable")
	ErrBadResponse = errors.New("invalid application API response")
)

// Stat is one label/value pair rendered on a card.
type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Control is one toggleable entity (Home Assistant) rendered on a card.
type Control struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	State string `json:"state"`
	On    bool   `json:"on"`
}

// Sensor is one environment reading (Home Assistant) rendered on a card.
type Sensor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
	Kind  string `json:"kind"`
}

// Widget is the normalized live data for one service.
type Widget struct {
	Type     string    `json:"type"`
	Stats    []Stat    `json:"stats"`
	Lines    []string  `json:"lines"`
	Controls []Control `json:"controls"`
	Sensors  []Sensor  `json:"sensors"`
}

// Provider resolves widgets for the dashboard's services, caching results
// briefly so open dashboards do not hammer the upstream applications.
type Provider struct {
	at            time.Time
	calendarAt    time.Time
	mediaAt       time.Time
	httpClient    *http.Client
	cached        map[string]*Widget
	calendarCache *CalendarResponse
	mediaCache    *MediaRail
	integrations  config.IntegrationsConfig
	mu            sync.Mutex
}

// New creates a widget Provider from the integrations config.
func New(integrations *config.IntegrationsConfig) *Provider {
	httpClient := new(http.Client)
	httpClient.Timeout = defaultTimeout

	provider := new(Provider)
	provider.httpClient = httpClient
	provider.integrations = *integrations
	provider.cached = make(map[string]*Widget)

	return provider
}

// Enabled reports whether any integration is fully configured.
func (provider *Provider) Enabled() bool {
	all := []config.IntegrationConfig{
		provider.integrations.Sonarr, provider.integrations.Radarr,
		provider.integrations.Jellyfin, provider.integrations.QBittorrent,
		provider.integrations.Immich, provider.integrations.Miniflux,
		provider.integrations.Forgejo, provider.integrations.Semaphore,
		provider.integrations.Paperless, provider.integrations.Syncthing,
		provider.integrations.HomeAssistant,
	}

	for _, integration := range all {
		if integration.Ready() {
			return true
		}
	}

	return false
}

// ForServices returns widget data keyed by service key for every service
// whose dashboard.yaml widget type matches a configured integration.
func (provider *Provider) ForServices(
	ctx context.Context,
	services []*dashboard.Service,
) map[string]*Widget {
	provider.mu.Lock()
	if time.Since(provider.at) < cacheTTL && provider.cached != nil {
		cached := provider.cached
		provider.mu.Unlock()

		return cached
	}
	provider.mu.Unlock()

	fresh := provider.fetchAll(ctx, services)

	provider.mu.Lock()
	provider.cached = fresh
	provider.at = time.Now()
	provider.mu.Unlock()

	return fresh
}

func (provider *Provider) fetchAll(
	ctx context.Context,
	services []*dashboard.Service,
) map[string]*Widget {
	out := make(map[string]*Widget)

	var (
		mu        sync.Mutex
		waitGroup sync.WaitGroup
	)

	for _, service := range services {
		if service.Widget == nil || service.Widget.Type == "" {
			continue
		}

		integration, ok := provider.integrationFor(service.Widget.Type)
		if !ok {
			continue
		}

		waitGroup.Go(func() {
			widget, err := provider.fetchWidget(ctx, service, integration)
			if err != nil {
				return
			}

			mu.Lock()
			out[service.Key] = widget
			mu.Unlock()
		})
	}

	waitGroup.Wait()

	return out
}

func (provider *Provider) integrationFor(widgetType string) (config.IntegrationConfig, bool) {
	integrations := &provider.integrations

	switch widgetType {
	case typeSonarr:
		return integrations.Sonarr, integrations.Sonarr.Ready()
	case typeRadarr:
		return integrations.Radarr, integrations.Radarr.Ready()
	case "jellyfin":
		return integrations.Jellyfin, integrations.Jellyfin.Ready()
	case "qbittorrent":
		return integrations.QBittorrent, integrations.QBittorrent.Ready()
	case "immich":
		return integrations.Immich, integrations.Immich.Ready()
	case "miniflux":
		return integrations.Miniflux, integrations.Miniflux.Ready()
	case "forgejo":
		return integrations.Forgejo, integrations.Forgejo.Ready()
	case "semaphore":
		return integrations.Semaphore, integrations.Semaphore.Ready()
	case "paperless":
		return integrations.Paperless, integrations.Paperless.Ready()
	case "syncthing":
		return integrations.Syncthing, integrations.Syncthing.Ready()
	case "homeassistant":
		return integrations.HomeAssistant, integrations.HomeAssistant.Ready()
	default:
		empty := new(config.IntegrationConfig)

		return *empty, false
	}
}

func (provider *Provider) fetchWidget(
	ctx context.Context,
	service *dashboard.Service,
	integration config.IntegrationConfig,
) (*Widget, error) {
	switch service.Widget.Type {
	case typeSonarr, typeRadarr:
		return provider.fetchArr(ctx, service.Widget.Type, integration)
	case "jellyfin":
		return provider.fetchJellyfin(ctx, integration)
	case "qbittorrent":
		return provider.fetchQBittorrent(ctx, integration)
	case "immich":
		return provider.fetchImmich(ctx, integration)
	case "miniflux":
		return provider.fetchMiniflux(ctx, integration)
	case "forgejo":
		return provider.fetchForgejo(ctx, integration)
	case "semaphore":
		return provider.fetchSemaphore(ctx, service, integration)
	case "paperless":
		return provider.fetchPaperless(ctx, integration)
	case "syncthing":
		return provider.fetchSyncthing(ctx, integration)
	case "homeassistant":
		return provider.fetchHomeAssistant(ctx, service, integration)
	default:
		return nil, fmt.Errorf(
			"%w: unsupported widget type %q",
			ErrUnavailable,
			service.Widget.Type,
		)
	}
}

type arrPaged struct {
	TotalRecords int `json:"totalRecords"`
}

func (provider *Provider) fetchArr(
	ctx context.Context,
	widgetType string,
	integration config.IntegrationConfig,
) (*Widget, error) {
	queue := new(arrPaged)

	err := provider.getJSON(ctx, integration, "/api/v3/queue?page=1&pageSize=1", queue)
	if err != nil {
		return nil, err
	}

	missing := new(arrPaged)

	err = provider.getJSON(ctx, integration, "/api/v3/wanted/missing?page=1&pageSize=1", missing)
	if err != nil {
		return nil, err
	}

	widget := new(Widget)
	widget.Type = widgetType
	widget.Stats = []Stat{
		{Label: "queue", Value: strconv.Itoa(queue.TotalRecords)},
		{Label: "missing", Value: strconv.Itoa(missing.TotalRecords)},
	}
	widget.Lines = []string{}

	return widget, nil
}

// Jellyfin answers with PascalCase keys; encoding/json matches these tags
// case-insensitively.
type jellyfinSession struct {
	NowPlaying *struct {
		Name       string  `json:"name"`
		SeriesName string  `json:"seriesName"`
		RunTime    float64 `json:"runTimeTicks"`
	} `json:"nowPlayingItem"`
	PlayState *struct {
		PositionTicks float64 `json:"positionTicks"`
	} `json:"playState"`
	UserName string `json:"userName"`
}

func (provider *Provider) fetchJellyfin(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	var sessions []jellyfinSession

	err := provider.getJSON(ctx, integration, "/Sessions", &sessions)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, maxLines)

	for _, session := range sessions {
		if session.NowPlaying == nil {
			continue
		}

		title := session.NowPlaying.Name
		if session.NowPlaying.SeriesName != "" {
			title = session.NowPlaying.SeriesName + " — " + title
		}

		line := title

		if session.PlayState != nil && session.NowPlaying.RunTime > 0 {
			percent := session.PlayState.PositionTicks / session.NowPlaying.RunTime * percentScale
			line = fmt.Sprintf("%s (%.0f%%)", line, percent)
		}

		if session.UserName != "" {
			line += " · " + session.UserName
		}

		lines = append(lines, line)
		if len(lines) >= maxLines {
			break
		}
	}

	widget := new(Widget)
	widget.Type = "jellyfin"
	widget.Stats = []Stat{{Label: "watching", Value: strconv.Itoa(len(lines))}}
	widget.Lines = lines

	return widget, nil
}

func (provider *Provider) getJSON(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath string,
	out any,
) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	fullURL := strings.TrimSuffix(integration.URL, "/") + apiPath

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", integration.APIKey)
	req.Header.Set("X-Emby-Token", integration.APIKey)
	req.Header.Set("X-Auth-Token", integration.APIKey)
	req.Header.Set("Authorization", "Bearer "+integration.APIKey)

	resp, err := provider.httpClient.Do(req)
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

func decodeJSON(content []byte, out any) error {
	err := json.Unmarshal(content, out)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return nil
}

// getJSONWithAuth is getJSON with one explicit Authorization header value,
// for APIs whose scheme is neither Bearer nor X-Api-Key (Paperless Token).
func (provider *Provider) getJSONWithAuth(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath, authorization string,
	out any,
) error {
	return provider.doGetJSON(
		ctx,
		integration,
		apiPath,
		map[string]string{"Authorization": authorization},
		out,
	)
}

// doGetJSON performs an authenticated GET with caller-supplied headers.
func (provider *Provider) doGetJSON(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath string,
	headers map[string]string,
	out any,
) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	fullURL := strings.TrimSuffix(integration.URL, "/") + apiPath

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := provider.httpClient.Do(req)
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

	return decodeJSON(content, out)
}

// postJSON sends a Bearer-authenticated JSON POST (Home Assistant services).
func (provider *Provider) postJSON(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath string,
	payload any,
) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return provider.doPost(
		ctx, integration, apiPath, "application/json",
		bytes.NewReader(content),
		map[string]string{"Authorization": "Bearer " + integration.APIKey},
	)
}

// postJSONBody sends an X-Api-Key JSON POST (Sonarr/Radarr adds).
func (provider *Provider) postJSONBody(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath string,
	payload any,
) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return provider.doPost(
		ctx, integration, apiPath, "application/json",
		bytes.NewReader(content),
		map[string]string{"X-Api-Key": integration.APIKey},
	)
}

// postFormCookie sends a cookie-authenticated form POST (qBittorrent).
func (provider *Provider) postFormCookie(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath, cookie string,
	form url.Values,
) error {
	return provider.doPost(
		ctx, integration, apiPath, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
		map[string]string{"Cookie": cookie},
	)
}

func (provider *Provider) doPost(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath, contentType string,
	body io.Reader,
	headers map[string]string,
) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	fullURL := strings.TrimSuffix(integration.URL, "/") + apiPath

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, fullURL, body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Content-Type", contentType)

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	_ = copyErr

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w: POST %s returned %s", ErrUnavailable, apiPath, resp.Status)
	}

	return nil
}
