package api

import (
	"fmt"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/session/auth"
	"github.com/zekihan/sdash/utils"
	"net/http"
)

func Login(w http.ResponseWriter, r *http.Request) {
	oidcKey := r.PathValue("oidc")
	if oidcKey == "" {
		http.NotFound(w, r)
		return
	}
	oidcSource, ok := auth.OidcSourceMap[oidcKey]
	if !ok {
		http.NotFound(w, r)
		return
	}

	redirected := r.URL.Query().Get("redirect")

	state, err := utils.RandString(16)
	routes.CheckError(w, err, http.StatusInternalServerError, "Failed to generate state")
	state = fmt.Sprintf("%s$$$%s", state, redirected)
	routes.SetCookie(w, r, "state", state)

	authURL := oidcSource.Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func LoginDefault(w http.ResponseWriter, r *http.Request) {
	redirected := r.URL.Query().Get("redirect")
	oidcSource, err := auth.GetDefaultOidcSource()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	url := fmt.Sprintf("/login/%s?redirect=%s", oidcSource.Key, redirected)
	http.Redirect(w, r, url, http.StatusSeeOther)
}
