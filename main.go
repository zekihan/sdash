package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/routes/icon"
	"github.com/zekihan/sdash/routes/middleware"
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
	config.GetDashboardConfig()

	icon.InitIconCache()

	// init middleware
	stdChain = middleware.New(middleware.Loging)
}

func main() {
	log.Infof("Starting SDash %s in %s", utils.GetVersion(), utils.GetEnvironment())

	go config.WatchConfigFile()

	http.Handle("/api/ping", stdChain.ThenFunc(routes.GetPing))
	http.Handle("/api/health", stdChain.ThenFunc(routes.GetHealth))
	http.Handle("/api/search", stdChain.ThenFunc(routes.GetSearch))

	fsServer := http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets")))
	http.Handle("/assets/", stdChain.ThenFunc(fsServer.ServeHTTP))

	http.Handle("/icon", stdChain.ThenFunc(icon.Proxy))
	http.Handle("/", stdChain.ThenFunc(routes.GetDashboard))

	port := config.GetConfig().WebConfig.Port
	host := config.GetConfig().WebConfig.Host
	log.Infof("Starting server on %s:%d", host, port)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
