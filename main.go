package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/justinas/alice"
	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/environment"
	"github.com/zekihan/go-commons/middleware"
	"github.com/zekihan/go-commons/utils"
	"github.com/zekihan/go-commons/web/errorroutes"
	"github.com/zekihan/go-commons/web/health"
	"github.com/zekihan/go-commons/web/ping"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes/api"
	"github.com/zekihan/sdash/routes/api/icon"
	"github.com/zekihan/sdash/routes/viewroutes"
	"github.com/zekihan/sdash/session/auth"
)

var (
	cfg       *config.Config
	stdChain  alice.Chain
	authChain alice.Chain
)

const (
	defaultTimeout = 10 * time.Second
)

func init() {
	err := environment.LoadEnvironmentFile()
	if err != nil {
		slog.ErrorContext(
			context.Background(),
			"Failed to load environment file",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	// Init config.
	cfg = config.GetConfigWithContext(context.Background())

	// Init logger.
	utils.InitLogger(&utils.LoggerConfig{
		Level:     cfg.LogConfig.Level,
		AddSource: cfg.LogConfig.AddSource,
	})

	cookie.InitSecureCookie(cfg.WebConfig.GetHashKey(), cfg.WebConfig.GetBlockKey())

	// Init oidc.
	auth.Init(cfg)

	// Init dashboard config.
	dashboard.GetConfigWithContext(context.Background())

	icon.InitIconCache()

	// Init middleware.
	stdChain = alice.New(middleware.RecoverHandler, middleware.Logging)
	authChain = alice.New(
		middleware.GetAuthMiddlewareForCookie(auth.IsAuthEnabled(), cfg.WebConfig.AuthCookie),
	)
}

func main() {
	slog.InfoContext(context.Background(), "Starting sdash server")
	utils.LogVersionInfo(context.Background())

	go dashboard.WatchConfigFile()

	// API routes.
	http.HandleFunc("/api/ping", ping.GetPing)
	http.HandleFunc("/api/health", health.GetHealth)
	http.Handle("/api/search", authChain.ThenFunc(api.GetSearch))
	http.Handle("/api/proxy/icon", authChain.ThenFunc(icon.Proxy))

	// Auth routes.
	http.HandleFunc("/login", api.LoginDefault)
	http.HandleFunc("/login/{oidc}", api.Login)
	http.HandleFunc("/user/oauth2/{oidc}/callback", api.AuthCallback)

	// Static files.
	fsServer := http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets")))
	http.HandleFunc("/assets/", fsServer.ServeHTTP)

	// Dashboard routes.
	http.Handle("/dashboard", authChain.ThenFunc(viewroutes.GetDashboard))
	http.Handle("/dashboard/group/{group}", authChain.ThenFunc(viewroutes.GetGroup))

	// Error routes.
	http.HandleFunc("/404", errorroutes.GetNotFound)
	http.HandleFunc("/5xx", errorroutes.Get5xx)

	http.HandleFunc("/", viewroutes.Root)

	port := cfg.WebConfig.Port
	host := cfg.WebConfig.Host
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	slog.InfoContext(context.Background(), "Starting server on http://"+addr)
	slog.InfoContext(
		context.Background(),
		fmt.Sprintf("Listening on %s:%d", host, port),
		slog.String("host", host),
		slog.Int("port", port),
		slog.String("baseUrl", cfg.WebConfig.BaseURL),
	)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      stdChain.Then(http.DefaultServeMux),
		ReadTimeout:  defaultTimeout,
		WriteTimeout: defaultTimeout,
	}

	err := srv.ListenAndServe()
	if err != nil {
		slog.ErrorContext(context.Background(), "Server failed to start", slog.Any("error", err))
		os.Exit(1)
	}
}
