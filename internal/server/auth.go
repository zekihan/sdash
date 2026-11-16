package server

import (
	"net/http"
	"sort"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/session"

	"github.com/zekihan/sdash/session/auth"
)

// requireAuth gates API routes with the OIDC session cookie. Browsers get a
// JSON 401 (the SPA renders its own login screen) rather than a redirect.
func (srv *Server) requireAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAuthEnabled() || srv.authenticated(r) {
			next(w, r)

			return
		}

		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func (srv *Server) authenticated(r *http.Request) bool {
	sess := new(session.Cookie)

	err := cookie.GetCookie(r, srv.deps.Config.WebConfig.AuthCookie, sess)

	return err == nil && sess.Authenticated && !sess.IsExpired()
}

type providerDTO struct {
	Key     string `json:"key"`
	Default bool   `json:"default"`
}

// handleMe tells the SPA whether auth is on, whether this browser is signed
// in, and which OIDC providers the login screen should offer.
func (srv *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	providers := make([]providerDTO, 0, len(auth.OidcSourceMap))

	defaultKey := ""

	defaultSource, err := auth.GetDefaultOidcSource()
	if err == nil {
		defaultKey = defaultSource.Key
	}

	for key := range auth.OidcSourceMap {
		provider := new(providerDTO)
		provider.Key = key
		provider.Default = key == defaultKey
		providers = append(providers, *provider)
	}

	sort.Slice(
		providers,
		func(first, second int) bool { return providers[first].Key < providers[second].Key },
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"authEnabled":   auth.IsAuthEnabled(),
		"authenticated": !auth.IsAuthEnabled() || srv.authenticated(r),
		"providers":     providers,
	})
}

// handleLogout clears the session cookie.
func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	name := srv.deps.Config.WebConfig.AuthCookie

	existing, err := r.Cookie(name)
	if err == nil {
		existing.Value = ""
		existing.MaxAge = -1
		existing.Path = "/"
		http.SetCookie(w, existing)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
