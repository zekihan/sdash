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

func TestPreferenceStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	store := NewPreferenceStore(path)

	prefs, err := store.Read()
	if err != nil || prefs != nil {
		t.Fatalf("Read() on missing file = %v, %v; want nil, nil", prefs, err)
	}

	err = store.Write([]byte(`{"theme":"dark"}`))
	if err != nil {
		t.Fatal(err)
	}

	prefs, err = store.Read()
	if err != nil || string(prefs) != `{"theme":"dark"}` {
		t.Fatalf("Read() = %q, %v; want stored document", prefs, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != prefsFileMode {
		t.Errorf("prefs file mode = %o, want %o", info.Mode().Perm(), prefsFileMode)
	}
}

func TestPreferenceStoreReadIgnoresInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")

	err := os.WriteFile(path, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	prefs, err := NewPreferenceStore(path).Read()
	if err != nil || prefs != nil {
		t.Fatalf("Read() = %v, %v; want nil, nil for invalid JSON", prefs, err)
	}
}

func TestPreferenceStoreWriteMissingDirectory(t *testing.T) {
	store := NewPreferenceStore(filepath.Join(t.TempDir(), "missing", "prefs.json"))

	err := store.Write([]byte(`{}`))
	if err == nil {
		t.Fatal("Write() into a missing directory must fail")
	}
}

func TestPreferencesHandlers(t *testing.T) {
	cfg := config.GetDefaultConfig()

	deps := new(Deps)
	deps.Config = cfg
	handler := New(deps).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/api/preferences", http.NoBody))

	var disabled map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &disabled)
	if err != nil {
		t.Fatal(err)
	}

	if disabled["enabled"] != false {
		t.Fatalf("GET without store = %v, want enabled=false", disabled)
	}

	deps.Prefs = NewPreferenceStore(filepath.Join(t.TempDir(), "prefs.json"))
	handler = New(deps).Handler()

	for _, body := range []string{"", "[1,2]", "{invalid"} {
		putRec := httptest.NewRecorder()
		handler.ServeHTTP(putRec, httptest.NewRequestWithContext(
			t.Context(), http.MethodPut, "/api/preferences", strings.NewReader(body)))

		if putRec.Code != http.StatusBadRequest {
			t.Errorf("PUT %q status = %d, want %d", body, putRec.Code, http.StatusBadRequest)
		}
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodPut, "/api/preferences", strings.NewReader(`{"theme":"light"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT valid prefs status = %d, want %d", rec.Code, http.StatusOK)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/api/preferences", http.NoBody))

	var enabled struct {
		Prefs   json.RawMessage `json:"prefs"`
		Enabled bool            `json:"enabled"`
	}

	err = json.Unmarshal(rec.Body.Bytes(), &enabled)
	if err != nil {
		t.Fatal(err)
	}

	if !enabled.Enabled || string(enabled.Prefs) != `{"theme":"light"}` {
		t.Errorf("GET after PUT = %s, want stored prefs", rec.Body.String())
	}
}
