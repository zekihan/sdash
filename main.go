package main

import (
	"fmt"
	"github.com/justinas/alice"
	log "github.com/sirupsen/logrus"
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
	"net/http"
	"os"
)

var cfg *config.Config
var stdChain alice.Chain
var authChain alice.Chain

func init() {
	err := environment.LoadEnvironmentFile()
	if err != nil {
		log.Errorf("Error loading environment file: %v", err)
		os.Exit(1)
	}

	// init config
	cfg = config.GetConfig()

	// Init logger.
	err = utils.InitLogger(&utils.LoggerConfig{
		Path:          cfg.LogConfig.Path,
		Level:         cfg.LogConfig.Level,
		AddMethodName: cfg.LogConfig.AddMethodName,
	})
	if err != nil {
		log.Errorf("Error initializing logger: %v", err)
		os.Exit(1)
	}

	cookie.InitSecureCookie(cfg.WebConfig.GetHashKey(), cfg.WebConfig.GetBlockKey())

	// init oidc
	auth.Init(cfg)

	// init dashboard config
	dashboard.GetConfig()

	icon.InitIconCache()

	// init middleware
	stdChain = alice.New(middleware.RecoverHandler, middleware.Logging)
	authChain = alice.New(middleware.GetAuthMiddlewareForCookie(auth.IsAuthEnabled(), cfg.WebConfig.AuthCookie))
}

func main() {
	log.Info("Starting sdash server")
	log.Info(utils.GetStartupInfo())

	go dashboard.WatchConfigFile()

	// API routes
	http.HandleFunc("/api/ping", ping.GetPing)
	http.HandleFunc("/api/health", health.GetHealth)
	http.Handle("/api/search", authChain.ThenFunc(api.GetSearch))
	http.Handle("/api/proxy/icon", authChain.ThenFunc(icon.Proxy))

	// Auth routes
	http.HandleFunc("/login", api.LoginDefault)
	http.HandleFunc("/login/{oidc}", api.Login)
	http.HandleFunc("/user/oauth2/{oidc}/callback", api.AuthCallback)

	// Static files
	fsServer := http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets")))
	http.HandleFunc("/assets/", fsServer.ServeHTTP)

	// Dashboard routes
	http.Handle("/dashboard", authChain.ThenFunc(viewroutes.GetDashboard))
	http.Handle("/dashboard/group/{group}", authChain.ThenFunc(viewroutes.GetGroup))

	// Error routes
	http.HandleFunc("/404", errorroutes.GetNotFound)
	http.HandleFunc("/5xx", errorroutes.Get5xx)

	http.HandleFunc("/", viewroutes.Root)

	port := cfg.WebConfig.Port
	host := cfg.WebConfig.Host
	log.Infof("Starting server on %s:%d", host, port)
	log.Infof("Url: %s", cfg.WebConfig.BaseURL)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), stdChain.Then(http.DefaultServeMux))
	if err != nil {
		log.Fatal(err)
	}
}
