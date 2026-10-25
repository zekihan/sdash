package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zekihan/go-commons/cookie"
	"github.com/zekihan/go-commons/middleware"
	"github.com/zekihan/go-commons/session"

	"github.com/zekihan/sdash/config"
)

func TestAuthenticatedSessionLifetime(t *testing.T) {
	for _, timeout := range []time.Duration{time.Second, 24 * time.Hour, 48 * time.Hour} {
		t.Run(timeout.String(), func(t *testing.T) {
			cfg := config.GetDefaultConfig().WebConfig
			cfg.SessionTimeout = timeout
			rec := httptest.NewRecorder()

			err := setAuthenticatedSession(rec, &cfg)
			if err != nil {
				t.Fatal(err)
			}

			req, sess := assertSessionCookie(t, rec, &cfg)

			protected := middleware.GetAuthMiddlewareForCookie(
				true,
				cfg.AuthCookie,
			)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			)
			before := httptest.NewRecorder()
			protected.ServeHTTP(before, req)

			if before.Code != http.StatusNoContent {
				t.Fatalf("valid session rejected: %d", before.Code)
			}

			if timeout == time.Second {
				assertSessionExpires(t, protected, req, sess.ExpireTime)
			}
		})
	}
}

func TestAuthenticatedSessionRejectsInvalidTimeout(t *testing.T) {
	cfg := config.GetDefaultConfig().WebConfig
	cfg.SessionTimeout = 0
	rec := httptest.NewRecorder()

	err := setAuthenticatedSession(rec, &cfg)
	if err == nil {
		t.Fatal("invalid timeout accepted")
	}

	if len(rec.Result().Cookies()) != 0 {
		t.Error("invalid timeout issued a cookie")
	}
}

func assertSessionCookie(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	cfg *config.WebConfig,
) (*http.Request, session.Cookie) {
	t.Helper()

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}

	authCookie := cookies[0]
	if authCookie.MaxAge != int(cfg.SessionTimeout/time.Second) || !authCookie.Secure ||
		!authCookie.HttpOnly ||
		authCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("unexpected cookie attributes: %+v", authCookie)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(authCookie)

	var sess session.Cookie

	err := cookie.GetCookie(req, cfg.AuthCookie, &sess)
	if err != nil {
		t.Fatal(err)
	}

	if !sess.Authenticated || sess.ExpireTime.Sub(sess.Created) != cfg.SessionTimeout {
		t.Errorf("unexpected session lifetime: %+v", sess)
	}

	if !authCookie.Expires.Equal(sess.ExpireTime.Truncate(time.Second)) {
		t.Error("browser and embedded expiries differ")
	}

	return req, sess
}

func assertSessionExpires(
	t *testing.T,
	protected http.Handler,
	req *http.Request,
	expiry time.Time,
) {
	t.Helper()

	timer := time.NewTimer(time.Until(expiry))
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal("test canceled")
	}

	after := httptest.NewRecorder()
	protected.ServeHTTP(after, req)

	if after.Code != http.StatusSeeOther {
		t.Errorf("expired session accepted: %d", after.Code)
	}
}
