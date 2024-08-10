package routes

import (
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard := config.GetDashboardConfig()
	err := views.Page(dashboard).Render(r.Context(), w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
