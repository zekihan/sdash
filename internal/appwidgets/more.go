package appwidgets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
)

const bytesPerUnit = 1024.0

func newWidget(widgetType string, stats []Stat, lines []string) *Widget {
	widget := new(Widget)
	widget.Type = widgetType
	widget.Stats = stats
	widget.Lines = lines
	widget.Controls = []Control{}
	widget.Sensors = []Sensor{}

	if widget.Lines == nil {
		widget.Lines = []string{}
	}

	return widget
}

func formatRate(bytesPerSecond int64) string {
	value := float64(bytesPerSecond)
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}

	unit := 0
	for value >= bytesPerUnit && unit < len(units)-1 {
		value /= bytesPerUnit
		unit++
	}

	return fmt.Sprintf("%.1f %s", value, units[unit])
}

// --- qBittorrent -----------------------------------------------------------.

// qBittorrent answers with snake_case keys.
//
//nolint:tagliatelle // upstream API uses snake_case
type qbitTransfer struct {
	DownloadSpeed int64 `json:"dl_info_speed"`
	UploadSpeed   int64 `json:"up_info_speed"`
}

type qbitTorrent struct {
	State string `json:"state"`
}

func (provider *Provider) fetchQBittorrent(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	cookie, err := provider.qbitLogin(ctx, integration)
	if err != nil {
		return nil, err
	}

	transfer := new(qbitTransfer)

	err = provider.getJSONCookie(ctx, integration, "/api/v2/transfer/info", cookie, transfer)
	if err != nil {
		return nil, err
	}

	var torrents []qbitTorrent

	err = provider.getJSONCookie(ctx, integration, "/api/v2/torrents/info", cookie, &torrents)
	if err != nil {
		return nil, err
	}

	active := 0

	for _, torrent := range torrents {
		if !strings.HasPrefix(torrent.State, "paused") {
			active++
		}
	}

	stats := []Stat{
		{Label: "down", Value: formatRate(transfer.DownloadSpeed)},
		{Label: "up", Value: formatRate(transfer.UploadSpeed)},
		{Label: "active", Value: strconv.Itoa(active)},
		{Label: "torrents", Value: strconv.Itoa(len(torrents))},
	}

	return newWidget("qbittorrent", stats, nil), nil
}

func (provider *Provider) qbitLogin(
	ctx context.Context,
	integration config.IntegrationConfig,
) (string, error) {
	if integration.Username == "" && integration.Password == "" {
		return "", nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("username", integration.Username)
	form.Set("password", integration.Password)

	loginURL := strings.TrimSuffix(integration.URL, "/") + "/api/v2/auth/login"

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodPost,
		loginURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	_ = copyErr

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: qBittorrent login returned %s", ErrUnavailable, resp.Status)
	}

	// QBittorrent 5 names the session cookie QBT_SID_<port> instead of SID.
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" || strings.HasPrefix(cookie.Name, "QBT_SID") {
			return cookie.Name + "=" + cookie.Value, nil
		}
	}

	return "", fmt.Errorf("%w: qBittorrent login set no SID cookie", ErrUnavailable)
}

func (provider *Provider) getJSONCookie(
	ctx context.Context,
	integration config.IntegrationConfig,
	apiPath, cookie string,
	out any,
) error {
	return provider.doGetJSON(ctx, integration, apiPath, map[string]string{"Cookie": cookie}, out)
}

// --- Immich ----------------------------------------------------------------.

type immichStatistics struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
	Total  int `json:"total"`
}

type immichJobs map[string]struct {
	JobCounts struct {
		Active int `json:"active"`
	} `json:"jobCounts"`
}

func (provider *Provider) fetchImmich(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	statistics := new(immichStatistics)

	err := provider.getJSON(ctx, integration, "/api/assets/statistics", statistics)
	if err != nil {
		return nil, err
	}

	stats := []Stat{
		{Label: "photos", Value: strconv.Itoa(statistics.Images)},
		{Label: "videos", Value: strconv.Itoa(statistics.Videos)},
	}

	jobs := new(immichJobs)

	err = provider.getJSON(ctx, integration, "/api/jobs", jobs)
	if err == nil {
		active := 0
		for _, job := range *jobs {
			active += job.JobCounts.Active
		}

		if active > 0 {
			stats = append(stats, Stat{Label: "jobs active", Value: strconv.Itoa(active)})
		}
	}

	return newWidget("immich", stats, nil), nil
}

// --- Miniflux ----------------------------------------------------------------.

type minifluxCounters struct {
	Unreads map[string]int `json:"unreads"`
}

func (provider *Provider) fetchMiniflux(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	counters := new(minifluxCounters)

	err := provider.getJSON(ctx, integration, "/v1/feeds/counters", counters)
	if err != nil {
		return nil, err
	}

	unread := 0
	for _, count := range counters.Unreads {
		unread += count
	}

	return newWidget("miniflux", []Stat{{Label: "unread", Value: strconv.Itoa(unread)}}, nil), nil
}

// --- Forgejo -----------------------------------------------------------------.

func (provider *Provider) fetchForgejo(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	stats := make([]Stat, 0)

	var notifications []jsonRaw

	err := provider.getJSON(
		ctx,
		integration,
		"/api/v1/notifications?status-types=unread&limit=50",
		&notifications,
	)
	if err == nil {
		stats = append(stats, Stat{Label: "notifications", Value: strconv.Itoa(len(notifications))})
	}

	var pulls []jsonRaw

	err = provider.getJSON(
		ctx,
		integration,
		"/api/v1/issues/search?state=open&type=pulls&created=true&limit=50",
		&pulls,
	)
	if err == nil {
		stats = append(stats, Stat{Label: "my open PRs", Value: strconv.Itoa(len(pulls))})
	}

	if len(stats) == 0 {
		return nil, fmt.Errorf("%w: forgejo returned no usable data", ErrUnavailable)
	}

	return newWidget("forgejo", stats, nil), nil
}

type jsonRaw = map[string]any

// --- Semaphore ---------------------------------------------------------------.

type semaphoreProject struct {
	ID int `json:"id"`
}

//nolint:tagliatelle // upstream API uses snake_case
type semaphoreTask struct {
	Status     string `json:"status"`
	Start      string `json:"start"`
	TemplateID int    `json:"template_id"`
}

type semaphoreTemplate struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

func (provider *Provider) fetchSemaphore(
	ctx context.Context,
	service *dashboard.Service,
	integration config.IntegrationConfig,
) (*Widget, error) {
	projectID, err := provider.semaphoreProjectID(ctx, service, integration)
	if err != nil {
		return nil, err
	}

	var tasks []semaphoreTask

	path := fmt.Sprintf("/api/project/%d/tasks?sort=start&order=desc&limit=10", projectID)

	err = provider.getJSON(ctx, integration, path, &tasks)
	if err != nil {
		return nil, err
	}

	// Semaphore ignores the limit parameter and returns its whole history.
	if len(tasks) > semaphoreRecentTasks {
		tasks = tasks[:semaphoreRecentTasks]
	}

	if len(tasks) == 0 {
		return newWidget("semaphore", []Stat{{Label: "runs", Value: "none yet"}}, nil), nil
	}

	successes := 0

	for _, task := range tasks {
		if task.Status == "success" {
			successes++
		}
	}

	latest := tasks[0]
	label := provider.semaphoreTemplateName(ctx, integration, projectID, latest.TemplateID)

	lines := []string{fmt.Sprintf("last: %s · %s", label, latest.Status)}

	return newWidget("semaphore", []Stat{
		{Label: "recent ok", Value: fmt.Sprintf("%d/%d", successes, len(tasks))},
	}, lines), nil
}

func (provider *Provider) semaphoreProjectID(
	ctx context.Context,
	service *dashboard.Service,
	integration config.IntegrationConfig,
) (int, error) {
	if service.Widget != nil && service.Widget.Options != nil {
		switch raw := service.Widget.Options["projectId"].(type) {
		case int:
			return raw, nil
		case int64:
			return int(raw), nil
		case float64:
			return int(raw), nil
		case string:
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr == nil {
				return parsed, nil
			}
		}
	}

	var projects []semaphoreProject

	err := provider.getJSON(ctx, integration, "/api/projects", &projects)
	if err != nil {
		return 0, err
	}

	if len(projects) == 0 {
		return 0, fmt.Errorf("%w: semaphore has no projects", ErrUnavailable)
	}

	return projects[0].ID, nil
}

func (provider *Provider) semaphoreTemplateName(
	ctx context.Context,
	integration config.IntegrationConfig,
	projectID, templateID int,
) string {
	var templates []semaphoreTemplate

	path := fmt.Sprintf("/api/project/%d/templates", projectID)

	err := provider.getJSON(ctx, integration, path, &templates)
	if err != nil {
		return fmt.Sprintf("template %d", templateID)
	}

	for _, template := range templates {
		if template.ID == templateID {
			return template.Name
		}
	}

	return fmt.Sprintf("template %d", templateID)
}

// --- Paperless ---------------------------------------------------------------.

//nolint:tagliatelle // upstream API uses snake_case
type paperlessStatistics struct {
	DocumentsTotal int `json:"documents_total"`
	InboxTotal     int `json:"documents_inbox_total"`
}

func (provider *Provider) fetchPaperless(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	statistics := new(paperlessStatistics)

	// Paperless expects "Token <key>", not Bearer; try Bearer-configured
	// request first via the shared helper is wrong here, so use a dedicated
	// call through getJSONWithAuth.
	err := provider.getJSONWithAuth(
		ctx,
		integration,
		"/api/statistics/",
		"Token "+integration.APIKey,
		statistics,
	)
	if err != nil {
		return nil, err
	}

	return newWidget("paperless", []Stat{
		{Label: "documents", Value: strconv.Itoa(statistics.DocumentsTotal)},
		{Label: "inbox", Value: strconv.Itoa(statistics.InboxTotal)},
	}, nil), nil
}

// --- Syncthing ---------------------------------------------------------------.

type syncthingCompletion struct {
	Completion float64 `json:"completion"`
}

type syncthingConnections struct {
	Connections map[string]struct {
		Connected bool `json:"connected"`
	} `json:"connections"`
}

func (provider *Provider) fetchSyncthing(
	ctx context.Context,
	integration config.IntegrationConfig,
) (*Widget, error) {
	completion := new(syncthingCompletion)

	err := provider.getJSON(ctx, integration, "/rest/db/completion", completion)
	if err != nil {
		return nil, err
	}

	stats := []Stat{{Label: "synced", Value: fmt.Sprintf("%.0f%%", completion.Completion)}}

	connections := new(syncthingConnections)

	err = provider.getJSON(ctx, integration, "/rest/system/connections", connections)
	if err == nil {
		connected := 0

		for _, connection := range connections.Connections {
			if connection.Connected {
				connected++
			}
		}

		stats = append(stats, Stat{
			Label: "devices",
			Value: fmt.Sprintf("%d/%d", connected, len(connections.Connections)),
		})
	}

	return newWidget("syncthing", stats, nil), nil
}

// --- Home Assistant ------------------------------------------------------------.

//nolint:tagliatelle // upstream API uses snake_case
type haState struct {
	EntityID   string `json:"entity_id"`
	State      string `json:"state"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
		DeviceClass  string `json:"device_class"`
		Unit         string `json:"unit_of_measurement"`
	} `json:"attributes"`
}

const maxSensors = 6

var sensorKinds = map[string]bool{
	"temperature": true, "humidity": true, "power": true,
	"energy": true, "pressure": true, "illuminance": true, "battery": true,
}

func (provider *Provider) fetchHomeAssistant(
	ctx context.Context,
	service *dashboard.Service,
	integration config.IntegrationConfig,
) (*Widget, error) {
	var states []haState

	err := provider.getJSON(ctx, integration, "/api/states", &states)
	if err != nil {
		return nil, err
	}

	lightsOn, lightsTotal, peopleHome, peopleTotal := 0, 0, 0, 0
	byID := make(map[string]haState, len(states))

	for _, state := range states {
		byID[state.EntityID] = state

		switch {
		case strings.HasPrefix(state.EntityID, "light."):
			lightsTotal++

			if state.State == "on" {
				lightsOn++
			}
		case strings.HasPrefix(state.EntityID, "person."):
			peopleTotal++

			if state.State == "home" {
				peopleHome++
			}
		}
	}

	stats := []Stat{
		{Label: "lights on", Value: fmt.Sprintf("%d/%d", lightsOn, lightsTotal)},
		{Label: "home", Value: fmt.Sprintf("%d/%d", peopleHome, peopleTotal)},
	}

	widget := newWidget("homeassistant", stats, nil)

	widget.Sensors = provider.haSensors(service, states)

	for _, entityID := range optionEntities(service) {
		state, ok := byID[entityID]
		if !ok {
			continue
		}

		control := new(Control)
		control.ID = entityID

		control.Label = state.Attributes.FriendlyName
		if control.Label == "" {
			control.Label = entityID
		}

		control.State = state.State
		control.On = state.State == "on" || state.State == "locked" || state.State == "playing"
		widget.Controls = append(widget.Controls, *control)
	}

	return widget, nil
}

// haSensors picks the configured sensor entities, or auto-detects
// environment sensors (temperature/humidity/power/...) When none are listed.
func (provider *Provider) haSensors(service *dashboard.Service, states []haState) []Sensor {
	wanted := optionStringList(service, "sensors")
	sensors := make([]Sensor, 0, maxSensors)

	appendSensor := func(state haState) {
		sensor := new(Sensor)
		sensor.ID = state.EntityID

		sensor.Label = state.Attributes.FriendlyName
		if sensor.Label == "" {
			sensor.Label = state.EntityID
		}

		sensor.Value = state.State
		sensor.Unit = state.Attributes.Unit
		sensor.Kind = state.Attributes.DeviceClass
		sensors = append(sensors, *sensor)
	}

	if len(wanted) > 0 {
		byID := make(map[string]haState, len(states))
		for _, state := range states {
			byID[state.EntityID] = state
		}

		for _, entityID := range wanted {
			if state, ok := byID[entityID]; ok {
				appendSensor(state)
			}
		}

		return sensors
	}

	for _, state := range states {
		if !sensorKinds[state.Attributes.DeviceClass] || state.State == "unavailable" ||
			state.State == "unknown" {
			continue
		}

		appendSensor(state)

		if len(sensors) >= maxSensors {
			break
		}
	}

	return sensors
}

func optionStringList(service *dashboard.Service, key string) []string {
	if service.Widget == nil || service.Widget.Options == nil {
		return nil
	}

	rawList, ok := service.Widget.Options[key].([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(rawList))

	for _, raw := range rawList {
		if text, isString := raw.(string); isString && text != "" {
			values = append(values, text)
		}
	}

	return values
}

func optionEntities(service *dashboard.Service) []string {
	return optionStringList(service, "entities")
}

// ToggleHomeAssistant toggles one entity via the homeassistant.toggle
// service. Callers must gate this behind the actions config.
func (provider *Provider) ToggleHomeAssistant(ctx context.Context, entityID string) error {
	integration := provider.integrations.HomeAssistant
	if !integration.Ready() {
		return fmt.Errorf("%w: homeassistant not configured", ErrUnavailable)
	}

	provider.invalidate()

	return provider.postJSON(ctx, integration, "/api/services/homeassistant/toggle",
		map[string]any{"entity_id": entityID})
}

func (provider *Provider) invalidate() {
	provider.mu.Lock()
	provider.at = time.Time{}
	provider.mu.Unlock()
}
