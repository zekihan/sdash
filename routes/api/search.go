package api

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/zekihan/go-commons/web"

	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/search"
	"github.com/zekihan/sdash/views"
)

func GetSearch(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfigWithContext(r.Context())
	result := config.ServicesList
	groups := config.GroupList

	query := r.URL.Query().Get("search")
	if query != "" {
		result = search.FuzzySearch(query, config)
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

	filteredDashboard := new(dashboard.Config)
	filteredDashboard.ServicesList = result
	filteredDashboard.GroupList = groups

	err := views.CardLists(filteredDashboard).Render(r.Context(), w)
	if errors.Is(err, context.Canceled) {
		return
	}

	web.CheckError(
		r.Context(),
		w,
		err,
		http.StatusInternalServerError,
		"failed to render card lists",
	)
}
