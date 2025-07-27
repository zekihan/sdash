package viewroutes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDashboard(t *testing.T) {
	// Test case: Successful rendering
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	GetDashboard(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
}
