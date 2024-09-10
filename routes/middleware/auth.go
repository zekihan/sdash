package middleware

import (
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/session"
	"github.com/zekihan/sdash/session/auth"
	"net/http"
)

func AuthHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAuthEnabled() {
			handler.ServeHTTP(w, r)
			return
		}
		sess, ok := getSessionFromCookie(r)
		if !ok || !sess.Authenticated || sess.IsExpired() {
			redirect(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func getSessionFromCookie(r *http.Request) (*session.Session, bool) {
	cookieKey := config.GetConfig().WebConfig.AuthCookie
	var cookie session.Session
	err := routes.GetCookie(r, cookieKey, &cookie)
	if err != nil {
		return nil, false
	}
	sess := session.Store.GetSession(cookie.ID)
	if sess == nil {
		return sess, false
	}
	return sess, true
}

func redirect(w http.ResponseWriter, r *http.Request, opts ...string) {
	path := "/login"
	currentPath := r.URL.Path

	switch len(opts) {
	case 1:
		path = opts[0]
	case 2:
		path = opts[0]
		currentPath = opts[1]
	}

	url := path
	if currentPath != "" {
		url += "?redirect=" + currentPath
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}
