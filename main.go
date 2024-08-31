package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes/api"
	"github.com/zekihan/sdash/routes/api/icon"
	"github.com/zekihan/sdash/routes/errorRoutes"
	"github.com/zekihan/sdash/routes/middleware"
	"github.com/zekihan/sdash/routes/viewRoutes"
	"github.com/zekihan/sdash/utils"
	"net/http"
	"os"
)

var stdChain middleware.Chain

func init() {
	// init config
	config.GetConfig()

	// init logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(config.GetConfig().LogConfig.Level)
	log.SetReportCaller(config.GetConfig().LogConfig.AddMethodName)

	// init dashboard config
	dashboard.GetConfig()

	icon.InitIconCache()

	// init middleware
	stdChain = middleware.New(middleware.RecoverHandler, middleware.Logging)
}

func main() {
	log.Infof("Starting SDash %s in %s", utils.GetVersion(), utils.GetEnvironment())

	go dashboard.WatchConfigFile()

	// API routes
	http.HandleFunc("/api/ping", api.GetPing)
	http.HandleFunc("/api/health", api.GetHealth)
	http.HandleFunc("/api/search", api.GetSearch)
	http.HandleFunc("/api/proxy/icon", icon.Proxy)

	// Static files
	fsServer := http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets")))
	http.HandleFunc("/assets/", fsServer.ServeHTTP)

	// Dashboard routes
	http.HandleFunc("/dashboard", viewRoutes.GetDashboard)
	http.HandleFunc("/dashboard/group/{group}", viewRoutes.GetGroup)

	// Error routes
	http.HandleFunc("/404", errorRoutes.GetNotFound)
	http.HandleFunc("/5xx", errorRoutes.Get5xx)

	// Redirect all other paths
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Redirect root path to dashboard
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusMovedPermanently)
			return
		}
		// Redirect all other paths to 404
		http.Redirect(w, r, "/404", http.StatusMovedPermanently)
	})

	port := config.GetConfig().WebConfig.Port
	host := config.GetConfig().WebConfig.Host
	log.Infof("Starting server on %s:%d", host, port)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), stdChain.Then(http.DefaultServeMux))
	if err != nil {
		log.Fatal(err)
	}
}
