package icon

import (
	"net/http"
	"strings"
	"testing"
)

// TestProxyRejectsSchemesAndPrivateHostsBeforeFetch proves URL-policy
// rejections happen before any outbound fetch: the fake transport fails the
// test if it is ever invoked.
func TestProxyRejectsSchemesAndPrivateHostsBeforeFetch(t *testing.T) {
	proxy := testProxy(func(*http.Request) (*http.Response, error) {
		t.Error("policy-rejected URL reached the transport")

		return nil, errTestDial
	})

	for _, raw := range []string{
		"file:///etc/passwd",
		"ftp://example.com/icon.png",
		"gopher://example.com/icon.png",
		"data:image/png;base64,AAAA",
		"javascript:alert(1)",
		"chrome://extensions/icon.png",
		"ws://example.com/icon.png",
		"//example.com/icon.png",
		"http://127.0.0.1/icon.png",
		"http://127.1/icon.png",
		"http://2130706433/icon.png",
		"http://0x7f000001/icon.png",
		"http://0x7f.0.0.1/icon.png",
		"http://0177.0.0.1/icon.png",
		"http://3232235521/icon.png",
		"http://10.1.2.3/icon.png",
		"http://172.31.255.1/icon.png",
		"http://192.168.0.1/icon.png",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/icon.png",
		"http://[fd00::1]/icon.png",
		"http://[fe80::1]/icon.png",
		"http://0.0.0.0/icon.png",
		"http://100.64.0.1/icon.png",
		"http://198.18.0.1/icon.png",
		"http://user:pass@example.com/icon.png",
		"http://example.com@10.0.0.1/icon.png",
		"http://example.com:8080/icon.png",
		"https://example.com:22/icon.png",
		"https:///icon.png",
		"http://:8080/icon.png",
		strings.Repeat("https://example.com/", 300),
	} {
		response := iconRequest(t, proxy, raw)
		if response.Code != http.StatusBadRequest {
			t.Errorf("url %q = %d, want 400", raw, response.Code)
		}

		if response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf(
				"url %q Cache-Control = %q, want no-store",
				raw,
				response.Header().Get("Cache-Control"),
			)
		}
	}
}

func TestParseIconURLNumericHostnameForms(t *testing.T) {
	for _, raw := range []string{
		"http://127.1/icon.png", "http://2130706433/icon.png",
		"http://0x7f000001/icon.png", "http://0177.0.0.1/icon.png",
		"http://0x/icon.png", "http://1.2.3.9999/icon.png",
	} {
		_, parseErr := parseIconURL(raw)
		if parseErr == nil {
			t.Errorf("parse %q succeeded, want rejection", raw)
		}
	}

	// Ordinary DNS names, including digit- and hex-letter labels, must pass.
	for _, raw := range []string{
		"http://example.com/icon.png", "http://123.example.com/icon.png",
		"http://dead.beef.example/icon.png", "http://8.8.8.8/icon.png",
		"https://cdn-1.example.org/icon.png",
	} {
		_, parseErr := parseIconURL(raw)
		if parseErr != nil {
			t.Errorf("parse %q rejected: %v", raw, parseErr)
		}
	}

	if !isNumericHostname("0X7F.0.0.1") || isNumericHostname("example.com") ||
		isNumericHostname("0xzz.example") || isNumericHostname("") ||
		isNumericHostname("0x.example.com") || !isNumericHostname("0x.1") {
		t.Error("isNumericHostname misclassified a hostname")
	}
}

// TestProxyRejectsNonImageAndOversizedAtHTTPBoundary exercises the full
// ServeHTTP path (not just readIcon) for content the transport returns.
func TestProxyRejectsNonImageAndOversizedAtHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		body        []byte
		length      int64
	}{
		{"html", "text/html", []byte("<html>hi</html>"), -2}, // length set below
		{"svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), -2},
		{"text as png", "image/png", []byte("not an image"), -2},
		{"png as jpeg", "image/jpeg", pngFixture(t), -2},
		{"oversized declared", "image/png", pngFixture(t), maxIconBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := testProxy(func(*http.Request) (*http.Response, error) {
				response := imageResponse(tc.body, tc.contentType)
				if tc.length != -2 {
					response.ContentLength = tc.length
				}

				return response, nil
			})

			response := iconRequest(t, proxy, "https://public.example/icon")
			if response.Code != http.StatusBadGateway {
				t.Errorf("%s = %d, want 502", tc.name, response.Code)
			}

			if response.Header().Get("Content-Type") == "image/png" &&
				response.Code == http.StatusOK {
				t.Error("rejected content served as image")
			}
		})
	}
}
