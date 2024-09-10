package viewroutes

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfig()
	err := views.Root(config).Render(r.Context(), w)
	if errors.As(err, &context.Canceled) {
		log.Debug("request cancelled")
		return
	}
	routes.CheckError(w, err, http.StatusInternalServerError, "failed to render dashboard")
}
