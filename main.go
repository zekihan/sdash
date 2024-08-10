package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/routes/middleware"
	"net/http"
	"os"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(config.GetConfig().LogConfig.Level)
	log.SetReportCaller(config.GetConfig().LogConfig.AddMethodName)

	stdChain := middleware.New(middleware.Loging)

	http.Handle("/ping", stdChain.ThenFunc(routes.GetPing))
	http.Handle("/health", stdChain.ThenFunc(routes.GetHealth))
	http.Handle("/", stdChain.ThenFunc(routes.GetDashboard))

	go config.WatchConfigFile()

	port := config.GetConfig().WebConfig.Port
	host := config.GetConfig().WebConfig.Host
	log.Infof("Starting server on %s:%d", host, port)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
