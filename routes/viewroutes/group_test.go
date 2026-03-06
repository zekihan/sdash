package viewroutes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetGroup(t *testing.T) {
	// Test case: Successful rendering
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/group", http.NoBody)
	req.SetPathValue("group", "infra")

	w := httptest.NewRecorder()
	GetGroup(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/group", http.NoBody)
	req.SetPathValue("group", "nonexistent")

	w := httptest.NewRecorder()
	GetGroup(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected status NotFound, got %v", w.Result().StatusCode)
	}
}
