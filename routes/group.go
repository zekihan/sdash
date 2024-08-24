package routes

import (
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetGroup(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfig()
	group := r.PathValue("group")
	if group == "" {
		http.NotFound(w, r)
		return
	}
	val, ok := config.Groups[group]
	if !ok {
		http.NotFound(w, r)
		return
	}
	results := make([]*dashboard.Service, 0)
	for _, service := range config.ServicesList {
		if service.Group == group {
			results = append(results, service)
		}
	}
	filteredDashboard := &dashboard.Config{ServicesList: results}
	err := views.Group(filteredDashboard, val).Render(r.Context(), w)
	if err != nil {
		log.Error("Error rendering dashboard: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("500 - Something bad happened!"))
		if err != nil {
			return
		}
	}
}
