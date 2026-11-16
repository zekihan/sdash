package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/environment"
	"github.com/zekihan/go-commons/middleware"
	"github.com/zekihan/go-commons/utils"

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
	"github.com/zekihan/sdash/internal/server"
	"github.com/zekihan/sdash/internal/truenasinfo"
	"github.com/zekihan/sdash/internal/weather"
	"github.com/zekihan/sdash/requestlog"
	"github.com/zekihan/sdash/session/auth"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
)

var startedAt = time.Now()

func main() {
	err := run()
	if err != nil {
		slog.Error("Server failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	err := environment.LoadEnvironmentFile()
	if err != nil {
		return fmt.Errorf("load environment file: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.GetConfigWithContext(ctx)

	loggerConfig := new(utils.LoggerConfig)
	loggerConfig.Level = cfg.LogConfig.Level
	loggerConfig.AddSource = cfg.LogConfig.AddSource
	utils.InitLogger(loggerConfig)

	cookie.InitSecureCookie(cfg.WebConfig.GetHashKey(), cfg.WebConfig.GetBlockKey())
	auth.Init(cfg)
	dashboard.GetConfigWithContext(ctx)
	icon.InitIconCache()

	slog.InfoContext(ctx, "Starting sdash server")
	utils.LogVersionInfo(ctx)

	go dashboard.WatchConfigFile()

	monitor := health.New(cfg.Health, func() []*dashboard.Service {
		return dashboard.GetConfigWithContext(ctx).ServicesList
	})

	deps := buildDeps(ctx, cfg, monitor)

	srv := server.New(deps)
	monitor.OnChange(srv.NotifyHealth)
	dashboard.OnReload(srv.NotifyConfig)

	go monitor.Run(ctx)

	addr := listenerAddress(cfg.WebConfig.Host, cfg.WebConfig.Port)

	httpServer := new(http.Server)
	httpServer.Addr = addr
	httpServer.Handler = middleware.RecoverHandler(requestlog.Logging(srv.Handler()))
	httpServer.ReadHeaderTimeout = readHeaderTimeout
	httpServer.IdleTimeout = idleTimeout

	go shutdownOnSignal(ctx, httpServer)

	slog.InfoContext(ctx, "Listening on http://"+addr,
		slog.String("baseUrl", cfg.WebConfig.BaseURL))

	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}

// buildDeps wires the optional widget clients from cfg. Disabled or
// misconfigured integrations stay nil and their widgets simply hide.
func buildDeps(ctx context.Context, cfg *config.Config, monitor *health.Monitor) *server.Deps {
	deps := new(server.Deps)
	deps.Config = cfg
	deps.Monitor = monitor
	deps.StartedAt = startedAt
	deps.Widgets = appwidgets.New(&cfg.Integrations)

	if cfg.Docker.Enabled {
		dockerClient, dockerErr := dockerinfo.New(cfg.Docker.Host)
		if dockerErr != nil {
			slog.WarnContext(ctx, "Docker widget unavailable", slog.Any("error", dockerErr))
		} else {
			deps.Docker = dockerClient
		}
	}

	if cfg.Weather.Enabled {
		deps.Weather = weather.New(cfg.Weather)
	}

	if cfg.Kubernetes.Enabled {
		kubeClient, kubeErr := kubeinfo.New(cfg.Kubernetes)
		if kubeErr != nil {
			slog.WarnContext(ctx, "Kubernetes widget unavailable", slog.Any("error", kubeErr))
		} else {
			deps.Kube = kubeClient
		}
	}

	if cfg.Proxmox.Enabled && cfg.Proxmox.URL != "" && cfg.Proxmox.APIToken != "" {
		deps.Proxmox = proxmoxinfo.New(cfg.Proxmox)
	}

	enableInfraWidgets(cfg, deps)

	return deps
}

// enableInfraWidgets wires the infrastructure-monitoring clients.
func enableInfraWidgets(cfg *config.Config, deps *server.Deps) {
	if cfg.PBS.Enabled && cfg.PBS.URL != "" && cfg.PBS.APIToken != "" {
		deps.PBS = pbsinfo.New(cfg.PBS)
	}

	if cfg.TrueNAS.Enabled && cfg.TrueNAS.URL != "" && cfg.TrueNAS.APIKey != "" {
		deps.TrueNAS = truenasinfo.New(cfg.TrueNAS)
	}

	if cfg.CrowdSec.Enabled && cfg.CrowdSec.URL != "" && cfg.CrowdSec.APIKey != "" {
		deps.CrowdSec = crowdsecinfo.New(cfg.CrowdSec)
	}

	if cfg.Preferences.Enabled {
		deps.Prefs = server.NewPreferenceStore(preferencesPath(cfg))
	}

	if cfg.Gatus.Enabled && cfg.Gatus.URL != "" {
		deps.Gatus = gatusinfo.New(cfg.Gatus.URL)
	}
}

// preferencesPath resolves the synced-preferences file: explicit config
// path, else sdash-prefs.json inside CONFIG_DIR (or the working directory).
func preferencesPath(cfg *config.Config) string {
	if cfg.Preferences.Path != "" {
		return cfg.Preferences.Path
	}

	dir := os.Getenv("CONFIG_DIR")
	if dir == "" {
		dir = "."
	}

	return filepath.Join(dir, "sdash-prefs.json")
}

func shutdownOnSignal(ctx context.Context, httpServer *http.Server) {
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	err := httpServer.Shutdown(shutdownCtx)
	if err != nil {
		slog.ErrorContext(ctx, "Graceful shutdown failed", slog.Any("error", err))
	}
}

func listenerAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
