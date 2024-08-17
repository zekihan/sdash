package routes

import (
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard := config.GetDashboardConfig()
	err := views.Root(dashboard).Render(r.Context(), w)
	if err != nil {
		log.Error("Error rendering dashboard: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("500 - Something bad happened!"))
		if err != nil {
			return
		}
	}
}
