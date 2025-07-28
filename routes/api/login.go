package api

import (
	"fmt"
	"net/http"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/utils"
	"github.com/zekihan/go-commons/web"

	"github.com/zekihan/sdash/session/auth"
)

const (
	defaultStateLength = 16
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

	state, err := utils.RandString(defaultStateLength)
	if web.CheckError(w, err, http.StatusInternalServerError, "Failed to generate state") {
		return
	}

	state = fmt.Sprintf("%s$$$%s", state, redirected)
	cookie.SetCookie(w, r, "state", state)

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
