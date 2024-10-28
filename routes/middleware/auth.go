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
		cookie, ok := getCookie(r)
		if !ok || !cookie.Authenticated || cookie.IsExpired() {
			redirect(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func getCookie(r *http.Request) (*session.Cookie, bool) {
	cookieKey := config.GetConfig().WebConfig.AuthCookie
	var cookie session.Cookie
	err := routes.GetCookie(r, cookieKey, &cookie)
	if err != nil {
		return nil, false
	}
	return &cookie, true
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

	isHx := false
	if r.Header.Get("HX-Request") == "true" {
		isHx = true
	}
	if !isHx {
		http.Redirect(w, r, url, http.StatusSeeOther)
	} else {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}
}
