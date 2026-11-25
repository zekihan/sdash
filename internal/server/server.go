// Package server wires the HTTP API, live event stream and embedded
// frontend for the dashboard.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/internal/appwidgets"
	"github.com/zekihan/sdash/internal/crowdsecinfo"
	"github.com/zekihan/sdash/internal/dockerinfo"
	"github.com/zekihan/sdash/internal/gatusinfo"
	"github.com/zekihan/sdash/internal/health"
	"github.com/zekihan/sdash/internal/icon"
	"github.com/zekihan/sdash/internal/kubeinfo"
	"github.com/zekihan/sdash/internal/pbsinfo"
	"github.com/zekihan/sdash/internal/proxmoxinfo"
	"github.com/zekihan/sdash/internal/systeminfo"
	"github.com/zekihan/sdash/internal/truenasinfo"
	"github.com/zekihan/sdash/internal/weather"
	"github.com/zekihan/sdash/session/auth"
	"github.com/zekihan/sdash/web"
)

const (
	sseHeartbeat   = 25 * time.Second
	sseBufferSize  = 16
	dockerTimeout  = 5 * time.Second
	staticCacheAge = "public, max-age=31536000, immutable"
)

// Deps are the server's collaborators. Any widget client may be nil when the
// feature is disabled; handlers then report the feature as unavailable.
type Deps struct {
	Config    *config.Config
	Monitor   *health.Monitor
	Docker    *dockerinfo.Client
	Weather   *weather.Client
	Kube      *kubeinfo.Client
	Proxmox   *proxmoxinfo.Client
	PBS       *pbsinfo.Client
	TrueNAS   *truenasinfo.Client
	CrowdSec  *crowdsecinfo.Client
	Prefs     *PreferenceStore
	Gatus     *gatusinfo.Client
	Widgets   *appwidgets.Provider
	StartedAt time.Time
}

// Server owns the mux and the SSE hub.
type Server struct {
	hub  *hub
	mux  *http.ServeMux
	deps Deps
}

// New builds a Server with all routes registered.
func New(deps *Deps) *Server {
	srv := new(Server)
	srv.deps = *deps
	srv.hub = newHub()
	srv.mux = http.NewServeMux()
	srv.routes()

	return srv
}

// Handler returns the root handler.
func (srv *Server) Handler() http.Handler {
	return srv.mux
}

// NotifyConfig pushes a config event to every connected browser.
func (srv *Server) NotifyConfig(*dashboard.Config) {
	srv.hub.broadcast(event{Type: "config", Data: map[string]bool{"reloaded": true}})
}

// NotifyHealth pushes fresh probe results to every connected browser.
func (srv *Server) NotifyHealth(statuses map[string]*health.Status) {
	srv.hub.broadcast(event{Type: "health", Data: statuses})
}

func (srv *Server) routes() {
	protected := func(handler http.HandlerFunc) http.Handler {
		return srv.requireAuth(handler)
	}
	crossOrigin := http.NewCrossOriginProtection()
	protectedWrite := func(handler http.HandlerFunc) http.Handler {
		return protected(crossOrigin.Handler(handler).ServeHTTP)
	}

	srv.mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
	})
	srv.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"uptimeSec": int64(time.Since(srv.deps.StartedAt).Seconds()),
		})
	})
	srv.mux.HandleFunc("GET /api/me", srv.handleMe)
	srv.mux.HandleFunc("GET /login", LoginDefault)
	srv.mux.HandleFunc("GET /login/{oidc}", Login)
	srv.mux.HandleFunc("GET /user/oauth2/{oidc}/callback", AuthCallback)
	srv.mux.Handle("GET /api/proxy/icon", protected(icon.Proxy))
	srv.mux.Handle("GET /api/dashboard", protected(srv.handleDashboard))
	srv.mux.Handle("GET /api/health/services", protected(srv.handleHealth))
	srv.mux.Handle("GET /api/system", protected(srv.handleSystem))
	srv.mux.Handle("GET /api/docker", protected(srv.handleDocker))
	srv.mux.Handle("GET /api/weather", protected(srv.handleWeather))
	srv.mux.Handle("GET /api/kubernetes", protected(srv.handleKubernetes))
	srv.mux.Handle("GET /api/proxmox", protected(srv.handleProxmox))
	srv.mux.Handle("GET /api/pbs", protected(srv.handlePBS))
	srv.mux.Handle("GET /api/truenas", protected(srv.handleTrueNAS))
	srv.mux.Handle("GET /api/crowdsec", protected(srv.handleCrowdSec))
	srv.mux.Handle("GET /api/preferences", protected(srv.handleGetPreferences))
	srv.mux.Handle("PUT /api/preferences", protectedWrite(srv.handlePutPreferences))
	srv.mux.Handle("POST /api/actions/argo-sync", protectedWrite(srv.handleArgoSync))
	srv.mux.Handle("POST /api/actions/restart", protectedWrite(srv.handleRestartDeployment))
	srv.mux.Handle("POST /api/actions/proxmox-start", protectedWrite(srv.handleProxmoxStart))
	srv.mux.Handle("POST /api/actions/wol", protectedWrite(srv.handleWakeOnLan))
	srv.mux.Handle("POST /api/actions/homeassistant", protectedWrite(srv.handleHomeAssistantToggle))
	srv.mux.Handle("POST /api/actions/qbittorrent-add", protectedWrite(srv.handleQBittorrentAdd))
	srv.mux.Handle("POST /api/actions/arr-add", protectedWrite(srv.handleArrAdd))
	srv.mux.Handle("GET /api/calendar", protected(srv.handleCalendar))
	srv.mux.Handle("GET /api/media-rail", protected(srv.handleMediaRail))
	srv.mux.Handle("GET /api/arr/lookup", protected(srv.handleArrLookup))
	srv.mux.Handle("GET /api/jellyfin/image/{id}", protected(srv.handleJellyfinImage))
	srv.mux.Handle("GET /api/immich/thumbnail/{id}", protected(srv.handleImmichThumbnail))
	srv.mux.Handle("GET /api/gatus", protected(srv.handleGatus))
	srv.mux.Handle("GET /api/service-widgets", protected(srv.handleServiceWidgets))
	srv.mux.Handle("GET /api/events", protected(srv.handleEvents))
	srv.mux.Handle("POST /api/logout", protected(srv.handleLogout))
	srv.mux.HandleFunc("/", srv.handleSPA)
}

type serviceDTO struct {
	Widget      *dashboard.Widget `json:"widget,omitempty"`
	Health      *health.Status    `json:"health,omitempty"`
	Key         string            `json:"key"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url"`
	Icon        string            `json:"icon,omitempty"`
	Group       string            `json:"group"`
	Location    string            `json:"location,omitempty"`
	Cluster     string            `json:"cluster,omitempty"`
	Tags        []string          `json:"tags"`
	OpenInNew   bool              `json:"openInNewTab"`
}

type groupDTO struct {
	Key      string       `json:"key"`
	Title    string       `json:"title"`
	Services []serviceDTO `json:"services"`
	Order    int          `json:"order"`
}

type settingsDTO struct {
	Title            string `json:"title"`
	Subtitle         string `json:"subtitle,omitempty"`
	SearchEngine     string `json:"searchEngine"`
	SearchEngineName string `json:"searchEngineName,omitempty"`
	Background       string `json:"background,omitempty"`
	DefaultTheme     string `json:"defaultTheme"`
	DefaultAccent    string `json:"defaultAccent"`
	DefaultDensity   string `json:"defaultDensity"`
	OpenInNewTab     bool   `json:"openInNewTab"`
	ShowClock        bool   `json:"showClock"`
}

type featuresDTO struct {
	Health     bool `json:"health"`
	System     bool `json:"system"`
	Docker     bool `json:"docker"`
	Weather    bool `json:"weather"`
	Auth       bool `json:"auth"`
	Kubernetes bool `json:"kubernetes"`
	Proxmox    bool `json:"proxmox"`
	PBS        bool `json:"pbs"`
	TrueNAS    bool `json:"truenas"`
	CrowdSec   bool `json:"crowdsec"`
	Actions    bool `json:"actions"`
	PrefsSync  bool `json:"prefsSync"`
	Gatus      bool `json:"gatus"`
	Widgets    bool `json:"widgets"`
}

type dashboardDTO struct {
	Settings settingsDTO `json:"settings"`
	Groups   []groupDTO  `json:"groups"`
	Features featuresDTO `json:"features"`
}

func (srv *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	cfg := dashboard.GetConfigWithContext(r.Context())

	var statuses map[string]*health.Status
	if srv.deps.Monitor != nil {
		statuses = srv.deps.Monitor.Snapshot()
	}

	byGroup := make(map[string][]serviceDTO)

	for _, service := range cfg.ServicesList {
		openInNew := cfg.Settings.OpenInNewTab != nil && *cfg.Settings.OpenInNewTab
		if service.OpenInNewTab != nil {
			openInNew = *service.OpenInNewTab
		}

		dto := serviceDTO{
			Key:         service.Key,
			Title:       service.Title,
			Description: service.Description,
			URL:         service.URL,
			Icon:        service.GetIconURL(),
			Group:       service.Group,
			Location:    service.Location,
			Cluster:     service.Cluster,
			Tags:        service.Tags,
			OpenInNew:   openInNew,
			Widget:      service.Widget,
			Health:      statuses[service.Key],
		}

		if dto.Tags == nil {
			dto.Tags = []string{}
		}

		byGroup[service.Group] = append(byGroup[service.Group], dto)
	}

	groups := make([]groupDTO, 0, len(cfg.GroupList))

	for _, group := range cfg.GroupList {
		services := byGroup[group.Key]
		if len(services) == 0 {
			continue
		}

		groups = append(groups, groupDTO{
			Key:      group.Key,
			Title:    group.Title,
			Order:    group.Order,
			Services: services,
		})
	}

	writeJSON(w, http.StatusOK, dashboardDTO{
		Settings: settingsDTO{
			Title:            cfg.Settings.Title,
			Subtitle:         cfg.Settings.Subtitle,
			SearchEngine:     cfg.Settings.SearchEngine,
			SearchEngineName: cfg.Settings.SearchEngineName,
			Background:       cfg.Settings.Background,
			DefaultTheme:     cfg.Settings.DefaultTheme,
			DefaultAccent:    cfg.Settings.DefaultAccent,
			DefaultDensity:   cfg.Settings.DefaultDensity,
			OpenInNewTab:     cfg.Settings.OpenInNewTab != nil && *cfg.Settings.OpenInNewTab,
			ShowClock:        cfg.Settings.ShowClock != nil && *cfg.Settings.ShowClock,
		},
		Features: featuresDTO{
			Health:     srv.deps.Config.Health.Enabled,
			System:     srv.deps.Config.System.Enabled,
			Docker:     srv.deps.Config.Docker.Enabled,
			Weather:    srv.deps.Config.Weather.Enabled,
			Auth:       auth.IsAuthEnabled(),
			Kubernetes: srv.deps.Kube != nil,
			Proxmox:    srv.deps.Proxmox != nil,
			PBS:        srv.deps.PBS != nil,
			TrueNAS:    srv.deps.TrueNAS != nil,
			CrowdSec:   srv.deps.CrowdSec != nil,
			Actions:    srv.deps.Config.Actions.Enabled,
			PrefsSync:  srv.deps.Prefs != nil,
			Gatus:      srv.deps.Gatus != nil,
			Widgets:    srv.deps.Widgets != nil && srv.deps.Widgets.Enabled(),
		},
		Groups: groups,
	})
}

func (srv *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if srv.deps.Monitor == nil {
		writeJSON(w, http.StatusOK, map[string]*health.Status{})

		return
	}

	writeJSON(w, http.StatusOK, srv.deps.Monitor.Snapshot())
}

func (srv *Server) handleSystem(w http.ResponseWriter, _ *http.Request) {
	if !srv.deps.Config.System.Enabled {
		writeJSON(w, http.StatusOK, new(systeminfo.Info))

		return
	}

	writeJSON(w, http.StatusOK, systeminfo.Read())
}

func (srv *Server) handleDocker(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Docker == nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "containers": []any{}})

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	containers, err := srv.deps.Docker.Containers(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "Docker list failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, map[string]any{
			"available":  false,
			"containers": []any{},
			"error":      "docker endpoint unavailable",
		})

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"available": true, "containers": containers})
}

func (srv *Server) handleWeather(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Weather == nil {
		writeError(w, http.StatusServiceUnavailable, "weather disabled")

		return
	}

	report, err := srv.deps.Weather.Current(r.Context())
	if err != nil {
		slog.WarnContext(r.Context(), "Weather fetch failed", slog.Any("error", err))
		writeError(w, http.StatusBadGateway, "weather unavailable")

		return
	}

	writeJSON(w, http.StatusOK, report)
}

type event struct {
	Data any
	Type string
}

type hub struct {
	clients map[chan event]struct{}
	mu      sync.RWMutex
}

func newHub() *hub {
	eventHub := new(hub)
	eventHub.clients = make(map[chan event]struct{})

	return eventHub
}

func (eventHub *hub) subscribe() chan event {
	channel := make(chan event, sseBufferSize)

	eventHub.mu.Lock()
	eventHub.clients[channel] = struct{}{}
	eventHub.mu.Unlock()

	return channel
}

func (eventHub *hub) unsubscribe(channel chan event) {
	eventHub.mu.Lock()
	delete(eventHub.clients, channel)
	eventHub.mu.Unlock()
}

func (eventHub *hub) broadcast(ev event) {
	eventHub.mu.RLock()
	defer eventHub.mu.RUnlock()

	for channel := range eventHub.clients {
		select {
		case channel <- ev:
		default: // slow client: drop the update, next round resyncs it
		}
	}
}

func (srv *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)

	err := controller.SetWriteDeadline(time.Time{})
	if err != nil {
		slog.DebugContext(r.Context(), "SSE write deadline unsupported", slog.Any("error", err))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	channel := srv.hub.subscribe()
	defer srv.hub.unsubscribe(channel)

	if srv.deps.Monitor != nil {
		writeEvent(w, controller, event{Type: "health", Data: srv.deps.Monitor.Snapshot()})
	}

	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, writeErr := w.Write([]byte(": ping\n\n"))
			if writeErr != nil {
				return
			}

			flush(controller)
		case ev := <-channel:
			writeEvent(w, controller, ev)
		}
	}
}

func flush(controller *http.ResponseController) {
	err := controller.Flush()
	if err != nil {
		slog.Debug("SSE flush failed", slog.Any("error", err))
	}
}

func writeEvent(w http.ResponseWriter, controller *http.ResponseController, ev event) {
	payload, err := json.Marshal(ev.Data)
	if err != nil {
		return
	}

	_, err = w.Write([]byte("event: " + ev.Type + "\ndata: " + string(payload) + "\n\n"))
	if err != nil {
		slog.Debug("SSE write failed", slog.Any("error", err))

		return
	}

	flush(controller)
}

var errNoFrontend = errors.New("frontend build missing")

func (srv *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not found")

		return
	}

	dist, err := web.DistFS()
	if err != nil {
		http.Error(w, "frontend not built: run `make web`", http.StatusInternalServerError)

		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if serveStaticFile(w, r, dist, name) {
		return
	}

	// Client-side route: serve the app shell.
	_, statErr := fs.Stat(dist, "index.html")
	if statErr != nil {
		http.Error(w, errNoFrontend.Error()+": run `make web`", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, dist, "index.html")
}

// serveStaticFile serves name from dist when it is an existing regular file.
// The embedded FS is rooted at dist and name must be a valid FS path, so a
// request can never escape the embedded tree.
func serveStaticFile(w http.ResponseWriter, r *http.Request, dist fs.FS, name string) bool {
	if name == "" || name == "." || !fs.ValidPath(name) {
		return false
	}

	info, err := fs.Stat(dist, name)
	if err != nil || info.IsDir() {
		return false
	}

	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", staticCacheAge)
	}

	if name == "sw.js" {
		w.Header().Set("Cache-Control", "no-cache")
	}

	if strings.HasSuffix(name, ".webmanifest") {
		w.Header().Set("Content-Type", "application/manifest+json")
	}

	//nolint:gosec // name is ValidPath-checked inside the embedded FS
	http.ServeFileFS(w, r, dist, name)

	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(value)
	if err != nil {
		slog.Error("Failed to encode response", slog.Any("error", err))
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
