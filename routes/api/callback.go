package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/session"

	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/session/auth"
)

var errSessionCreation = errors.New("failed to create session")

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

	redirected := stateParsed[1]
	if redirected == "" || !strings.HasPrefix(redirected, "/") {
		redirected = "/"
	}

	ctx := r.Context()

	oauth2Token, err := oidcSource.Config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to exchange token")
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)

		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token field in oauth2 token", http.StatusBadRequest)

		return
	}

	idToken, err := oidcSource.Verify(ctx, rawIDToken)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to verify ID token")
		http.Error(w, "Failed to verify ID token", http.StatusInternalServerError)

		return
	}

	var profile map[string]any

	err = idToken.Claims(&profile)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get claims")
		http.Error(w, "Failed to get claims", http.StatusInternalServerError)

		return
	}

	webConfig := &config.GetConfigWithContext(r.Context()).WebConfig

	err = setAuthenticatedSession(w, webConfig)
	if err != nil {
		http.Error(w, "failed to set session cookie", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, redirected, http.StatusSeeOther)
}

func setAuthenticatedSession(w http.ResponseWriter, cfg *config.WebConfig) error {
	err := cfg.ValidateSessionTimeout()
	if err != nil {
		return fmt.Errorf("invalid session timeout: %w", err)
	}

	sess := session.NewCookie()
	if sess == nil {
		return errSessionCreation
	}

	sess.Authenticated = true
	sess.ExpireTime = sess.Created.Add(cfg.SessionTimeout)

	authCookie, err := cookie.CreateNewCookie(cfg.AuthCookie, sess)
	if err != nil {
		return fmt.Errorf("create auth cookie: %w", err)
	}

	authCookie.MaxAge = int(cfg.SessionTimeout / time.Second)
	authCookie.Expires = sess.ExpireTime
	http.SetCookie(w, authCookie)

	return nil
}
