package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
)

func iconProxyRequest(
	t *testing.T,
	handler http.Handler,
	rawURL string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(),
		http.MethodGet,
		"/api/proxy/icon?url="+url.QueryEscape(rawURL),
		http.NoBody,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestIconProxyRejectsPrivateAndNonPublicDestinations(t *testing.T) {
	cfg := config.GetDefaultConfig()
	handler := New(actionDeps(cfg)).Handler()

	cases := []string{
		"", // missing url
		"not-a-url",
		"http://127.0.0.1/icon.png",
		"http://127.0.0.1:80/icon.png",
		"https://127.0.0.1/icon.png",
		"http://10.0.0.5/icon.png",
		"http://172.16.0.5/icon.png",
		"http://192.168.1.10/icon.png",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/icon.png",
		"http://[::ffff:127.0.0.1]/icon.png",
		"http://0.0.0.0/icon.png",
		"file:///etc/passwd",
		"ftp://example.com/icon.png",
		"gopher://example.com/icon.png",
		"data:image/png;base64,iVBORw0KGgo=",
		"javascript:alert(1)",
		"//example.com/icon.png",
		"http://user:pass@example.com/icon.png",
		"http://example.com@127.0.0.1/icon.png",
		"http://example.com:8080/icon.png",
		"https://example.com:80/icon.png",
		"http://example.com:443/icon.png",
		"https://example.com:abc/icon.png",
	}
	for _, raw := range cases {
		rec := iconProxyRequest(t, handler, raw)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("icon url %q = %d, want 400 (body %q)", raw, rec.Code, rec.Body.String())
		}

		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("icon url %q missing nosniff header", raw)
		}

		if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'none'") {
			t.Errorf(
				"icon url %q missing CSP header: %q",
				raw,
				rec.Header().Get("Content-Security-Policy"),
			)
		}

		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf(
				"icon url %q Cache-Control = %q, want no-store on rejection",
				raw,
				rec.Header().Get("Cache-Control"),
			)
		}
	}

	oversized := "https://example.com/" + strings.Repeat("a", 5000)
	if rec := iconProxyRequest(t, handler, oversized); rec.Code != http.StatusBadRequest {
		t.Errorf("oversized icon url = %d, want 400", rec.Code)
	}
}

// Encoded loopback forms (hex/octal/decimal/short) are not IP literals to
// netip, so parseIconURL passes them to DNS. The dialer must still refuse:
// Go's resolver maps them to 127.0.0.1 and publicDialer rejects the answer.
// End-to-end the proxy must fail closed (502), never return an image.
func TestIconProxyEncodedLoopbackFailsClosed(t *testing.T) {
	cfg := config.GetDefaultConfig()
	handler := New(actionDeps(cfg)).Handler()

	for _, raw := range []string{
		"http://0x7f.0.0.1/icon.png",
		"http://0177.0.0.1/icon.png",
		"http://2130706433/icon.png",
		"http://127.1/icon.png",
		"http://localhost/icon.png",
	} {
		rec := iconProxyRequest(t, handler, raw)
		if rec.Code == http.StatusOK {
			t.Errorf(
				"encoded loopback %q returned 200 with %d bytes; want fail-closed",
				raw,
				rec.Body.Len(),
			)
		}

		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusBadGateway {
			t.Errorf("encoded loopback %q = %d, want 400 or 502", raw, rec.Code)
		}
	}
}

func TestIconProxyUnresolvablePublicHostFailsClosed(t *testing.T) {
	cfg := config.GetDefaultConfig()
	handler := New(actionDeps(cfg)).Handler()

	// .example is a reserved TLD, so DNS cannot resolve this in any environment.
	rec := iconProxyRequest(t, handler, "https://public.example/icon.png")
	if rec.Code != http.StatusBadGateway {
		t.Errorf("unresolvable public icon = %d, want 502 (body %q)", rec.Code, rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "icon") &&
		rec.Header().Get("Content-Type") == "image/png" {
		t.Error("failed fetch must not be served as an image")
	}
}
