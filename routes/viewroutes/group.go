package viewroutes

import (
	"context"
	"errors"
	"net/http"

	"github.com/zekihan/go-commons/web"

	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/views"
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
	if errors.Is(err, context.Canceled) {
		return
	}

	web.CheckError(w, err, http.StatusInternalServerError, "failed to render dashboard")
}
