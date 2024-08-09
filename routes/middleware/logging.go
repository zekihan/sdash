package middleware

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func Loging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info(r.Method, " ", r.URL.Path, " ", r.RemoteAddr)
		handler.ServeHTTP(w, r)
	})
}
