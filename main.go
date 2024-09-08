package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes/api"
	"github.com/zekihan/sdash/routes/api/icon"
	"github.com/zekihan/sdash/routes/errorroutes"
	"github.com/zekihan/sdash/routes/middleware"
	"github.com/zekihan/sdash/routes/viewroutes"
	"github.com/zekihan/sdash/session"
	"github.com/zekihan/sdash/session/auth"
	"github.com/zekihan/sdash/utils"
	"net/http"
)

var cfg *config.Config
var stdChain middleware.Chain
var authChain middleware.Chain

func init() {
	// init config
	cfg = config.GetConfig()

	// init logger
	utils.InitLogger(cfg)

	// init oidc
	auth.Init(cfg)

	// init session
	session.Init(cfg)

	// init dashboard config
	dashboard.GetConfig()

	icon.InitIconCache()

	// init middleware
	stdChain = middleware.New(middleware.RecoverHandler, middleware.Logging)
	authChain = middleware.New(middleware.AuthHandler)
}

func main() {
	log.Infof("Starting SDash %s in %s", utils.GetVersion(), utils.GetEnvironment())

	go dashboard.WatchConfigFile()

	// API routes
	http.HandleFunc("/api/ping", api.GetPing)
	http.HandleFunc("/api/health", api.GetHealth)
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
