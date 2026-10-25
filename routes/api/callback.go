package api

import (
	"net/http"
	"strings"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/session"
	"github.com/zekihan/go-commons/web"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

func AuthCallback(w http.ResponseWriter, r *http.Request) {
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

	var state string

	err := cookie.GetCookie(r, "state", &state)
	if err != nil {
		http.Error(w, "state not found", http.StatusBadRequest)

		return
	}

	if r.URL.Query().Get("state") != state {
		http.Error(w, "state did not match", http.StatusBadRequest)

		return
	}

	stateParsed := strings.SplitN(
		state,
		"$$$",
		2, //nolint:mnd // split into two parts: oidcKey and redirected URL
	)
	if len(
		stateParsed,
	) != 2 { //nolint:mnd // split into two parts: oidcKey and redirected URL
		http.Error(w, "state is invalid", http.StatusBadRequest)

		return
	}

	redirected := localRedirectPath(stateParsed[1])

	ctx := r.Context()

	oauth2Token, err := oidcSource.Config.Exchange(ctx, r.URL.Query().Get("code"))
	if web.CheckError(
		r.Context(),
		w,
		err,
		http.StatusInternalServerError,
		"Failed to exchange token",
	) {
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token field in oauth2 token", http.StatusBadRequest)

		return
	}

	idToken, err := oidcSource.Verify(ctx, rawIDToken)
	if web.CheckError(
		r.Context(),
		w,
		err,
		http.StatusInternalServerError,
		"Failed to verify ID token",
	) {
		return
	}

	var profile map[string]any

	err = idToken.Claims(&profile)
	if web.CheckError(r.Context(), w, err, http.StatusInternalServerError, "Failed to get claims") {
		return
	}

	sess := session.NewCookie()
	if sess == nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)

		return
	}

	sess.Authenticated = true

	cookieName := config.GetConfigWithContext(r.Context()).WebConfig.AuthCookie

	err = cookie.SetCookie(w, r, cookieName, sess)
	if err != nil {
		http.Error(w, "failed to set cookie", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, redirected, http.StatusSeeOther)
}
