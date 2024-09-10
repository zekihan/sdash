package api

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/search"
	"github.com/zekihan/sdash/views"
	"net/http"
	"slices"
)

func GetSearch(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfig()
	result := config.ServicesList
	groups := config.GroupList

	query := r.URL.Query().Get("search")
	if query != "" {
		result = search.FuzzySearch(query, result)
		result.Sort(config.OrderServicesVia)
		groups = make(dashboard.GroupList, 0)
		for _, service := range result {
			if !slices.ContainsFunc(groups, func(group *dashboard.Group) bool {
				return group.Key == service.Group
			}) {
				groups = append(groups, config.Groups[service.Group])
			}
		}
		groups.Sort(config.OrderGroupsVia)
	}
	filteredDashboard := &dashboard.Config{ServicesList: result, GroupList: groups}
	err := views.CardLists(filteredDashboard).Render(r.Context(), w)
	if errors.As(err, &context.Canceled) {
		log.Debug("request cancelled")
		return
	}
	routes.CheckError(w, err, http.StatusInternalServerError, "failed to render card lists")
}
