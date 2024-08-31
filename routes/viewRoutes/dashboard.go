package viewRoutes

import (
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfig()
	err := views.Root(config).Render(r.Context(), w)
	routes.CheckError(w, err, http.StatusInternalServerError, "failed to render dashboard")
}
