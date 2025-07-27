package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/session"
	"github.com/zekihan/sdash/session/auth"
)

func TestAuthHandler_NoAuthEnabled(t *testing.T) {
	auth.Enabled = false
	handler := AuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthHandler_AuthEnabled_Unauthenticated(t *testing.T) {
	auth.Enabled = true
	handler := AuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/login")
}

func TestAuthHandler_AuthEnabled_Authenticated(t *testing.T) {
	auth.Enabled = true
	handler := AuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	sessionCookie := session.Cookie{
		Authenticated: true,
		ExpireTime:    time.Now().Add(time.Hour),
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	cookie, err := routes.CreateNewCookie(config.GetConfig().WebConfig.AuthCookie, sessionCookie)
	if err != nil {
		t.Fatalf("Failed to create cookie: %v", err)
	}
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/some-path", nil)
	rec := httptest.NewRecorder()

	redirect(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/login?redirect=/some-path")
}

func TestRedirectFromHx(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/some-path", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	redirect(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("HX-Refresh"), "true")
}

func TestGetCookie_ValidCookie(t *testing.T) {
	sessionCookie := session.Cookie{
		Authenticated: true,
		ExpireTime:    time.Now().Add(-time.Hour),
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	cookie, err := routes.CreateNewCookie(config.GetConfig().WebConfig.AuthCookie, sessionCookie)
	if err != nil {
		t.Fatalf("Failed to create cookie: %v", err)
	}
	req.AddCookie(cookie)
	result, ok := getCookie(req)
	assert.True(t, ok)
	assert.Equal(t, sessionCookie.Authenticated, result.Authenticated)
}

func TestGetCookie_InvalidCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	result, ok := getCookie(req)
	assert.False(t, ok)
	assert.Nil(t, result)
}
