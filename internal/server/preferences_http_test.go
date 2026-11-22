package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config"
)

func prefsHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prefs.json")
	cfg := config.GetDefaultConfig()
	deps := actionDeps(cfg)
	deps.Prefs = NewPreferenceStore(path)

	return New(deps).Handler(), path
}

func putPrefs(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		"/api/preferences",
		strings.NewReader(body),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func getPrefs(t *testing.T, handler http.Handler) (bool, json.RawMessage) {
	t.Helper()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/api/preferences",
		http.NoBody,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET preferences = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Prefs   json.RawMessage `json:"prefs"`
		Enabled bool            `json:"enabled"`
	}

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode GET preferences: %v", err)
	}

	return body.Enabled, body.Prefs
}

func TestPreferencesPutValidation(t *testing.T) {
	handler, path := prefsHandler(t)

	invalid := map[string]string{
		"empty":           "",
		"whitespace only": "  \n\t ",
		"array":           "[1,2]",
		"null":            "null",
		"number":          "42",
		"string":          `"dark"`,
		"bool":            "true",
		"truncated":       `{"theme":`,
		"trailing junk":   `{"theme":"dark"} junk`,
	}
	for name, body := range invalid {
		if rec := putPrefs(t, handler, body); rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %s (%q) = %d, want 400", name, body, rec.Code)
		}
	}

	_, statErr := os.Stat(path)
	if !os.IsNotExist(statErr) {
		t.Errorf("invalid PUTs created the prefs file (stat err=%v)", statErr)
	}

	// Leading whitespace is still one JSON object document.
	if rec := putPrefs(t, handler, "  \n {\"theme\":\"dark\"}"); rec.Code != http.StatusOK {
		t.Errorf(
			"PUT whitespace-prefixed object = %d, want 200 (body %q)",
			rec.Code,
			rec.Body.String(),
		)
	}

	enabled, prefs := getPrefs(t, handler)
	if !enabled || strings.TrimSpace(string(prefs)) != `{"theme":"dark"}` {
		// Stored bytes may keep the caller's whitespace; compare decoded.
		var decoded map[string]string

		err := json.Unmarshal(prefs, &decoded)
		if err != nil || decoded["theme"] != "dark" {
			t.Errorf(
				"GET after whitespace PUT = enabled=%v prefs=%s, want theme=dark",
				enabled,
				prefs,
			)
		}
	}
}

func TestPreferencesSizeLimits(t *testing.T) {
	handler, _ := prefsHandler(t)

	if rec := putPrefs(t, handler, `{"theme":"light"}`); rec.Code != http.StatusOK {
		t.Fatalf("seed PUT = %d, want 200", rec.Code)
	}

	// Just over the 64 KiB cap: truncated by the LimitReader and rejected;
	// the previously stored document must survive untouched.
	oversized := `{"data":"` + strings.Repeat("x", maxPrefsBytes) + `"}`
	if rec := putPrefs(t, handler, oversized); rec.Code != http.StatusBadRequest {
		t.Errorf("PUT oversized (%d bytes) = %d, want 400", len(oversized), rec.Code)
	}

	_, prefs := getPrefs(t, handler)
	if string(prefs) != `{"theme":"light"}` {
		t.Errorf("GET after oversized PUT = %s, want previous document preserved", prefs)
	}

	// A large-but-under-cap object is accepted and returned verbatim.
	large := `{"data":"` + strings.Repeat("y", 32<<10) + `"}`
	if rec := putPrefs(t, handler, large); rec.Code != http.StatusOK {
		t.Fatalf("PUT 32KiB object = %d, want 200", rec.Code)
	}

	_, prefs = getPrefs(t, handler)
	if string(prefs) != large {
		t.Errorf("GET after large PUT returned %d bytes, want the stored document", len(prefs))
	}
}

func TestPreferencesPutDisabledAndStoreFailure(t *testing.T) {
	cfg := config.GetDefaultConfig()
	handler := New(actionDeps(cfg)).Handler() // Prefs nil

	if rec := putPrefs(t, handler, `{"theme":"dark"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("PUT without store = %d, want 503", rec.Code)
	}

	enabled, prefs := getPrefs(t, handler)
	if enabled || string(prefs) != "null" {
		t.Errorf(
			"GET without store = enabled=%v prefs=%s, want enabled=false prefs=null",
			enabled,
			prefs,
		)
	}

	// Store pointing into a missing directory: PUT surfaces a 500, GET
	// degrades to enabled with null prefs rather than an error status.
	brokenCfg := config.GetDefaultConfig()
	brokenDeps := actionDeps(brokenCfg)
	brokenDeps.Prefs = NewPreferenceStore(filepath.Join(t.TempDir(), "missing", "prefs.json"))
	broken := New(brokenDeps).Handler()

	if rec := putPrefs(t, broken, `{"theme":"dark"}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("PUT with unwritable store = %d, want 500", rec.Code)
	}

	enabled, prefs = getPrefs(t, broken)
	if !enabled || string(prefs) != "null" {
		t.Errorf(
			"GET with unreadable store = enabled=%v prefs=%s, want enabled=true prefs=null",
			enabled,
			prefs,
		)
	}
}

func TestPreferencesOverwriteRoundTrip(t *testing.T) {
	handler, _ := prefsHandler(t)

	enabled, prefs := getPrefs(t, handler)
	if !enabled || string(prefs) != "null" {
		t.Fatalf("initial GET = enabled=%v prefs=%s, want enabled=true prefs=null", enabled, prefs)
	}

	for _, doc := range []string{`{"theme":"dark","accent":"blue"}`, `{"theme":"light"}`} {
		if rec := putPrefs(t, handler, doc); rec.Code != http.StatusOK {
			t.Fatalf("PUT %s = %d, want 200", doc, rec.Code)
		}

		_, prefs = getPrefs(t, handler)
		if string(prefs) != doc {
			t.Errorf("GET = %s, want stored %s", prefs, doc)
		}
	}
}
