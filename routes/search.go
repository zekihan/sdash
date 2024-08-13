package routes

import (
	"github.com/sahilm/fuzzy"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
	"slices"
	"strings"
)

type Services []config.DashboardService

func GetSearch(w http.ResponseWriter, r *http.Request) {
	dashboard := config.GetDashboardConfig()
	result := dashboard.ServicesList

	query := r.URL.Query().Get("search")
	if query != "" {
		result = FuzzySearch(query, result)
	}
	filteredDashboard := &config.DashboardConfig{ServicesList: result}
	filteredDashboard.GroupList = filteredDashboard.GetGroupsFromList()
	err := views.CardLists(filteredDashboard).Render(r.Context(), w)
	if err != nil {
		log.Error("Error rendering dashboard: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("500 - Something bad happened!"))
		if err != nil {
			return
		}
	}
}

func FuzzySearch(query string, list Services) Services {
	titles := make([]string, len(list))
	groups := make([]string, len(list))
	locations := make([]string, len(list))
	clusters := make([]string, len(list))
	tags := make([]string, len(list))
	for i, service := range list {
		titles[i] = service.Title
		groups[i] = service.Group
		locations[i] = service.Location
		clusters[i] = service.Cluster
		tags[i] = strings.Join(service.Tags, "§§§")
	}
	titlesMatches := fuzzy.FindNoSort(query, titles)
	titlesResults := make([]config.DashboardService, len(titlesMatches))
	for i, match := range titlesMatches {
		titlesResults[i] = list[match.Index]
	}
	groupsMatches := fuzzy.FindNoSort(query, groups)
	groupsResults := make([]config.DashboardService, len(groupsMatches))
	for i, match := range groupsMatches {
		groupsResults[i] = list[match.Index]
	}
	locationsMatches := fuzzy.FindNoSort(query, locations)
	locationsResults := make([]config.DashboardService, len(locationsMatches))
	for i, match := range locationsMatches {
		locationsResults[i] = list[match.Index]
	}
	clustersMatches := fuzzy.FindNoSort(query, clusters)
	clustersResults := make([]config.DashboardService, len(clustersMatches))
	for i, match := range clustersMatches {
		clustersResults[i] = list[match.Index]
	}
	tagsMatches := fuzzy.FindNoSort(query, tags)
	tagsResults := make([]config.DashboardService, len(tagsMatches))
	for i, match := range tagsMatches {
		tagsResults[i] = list[match.Index]
	}
	results := make([]config.DashboardService, 0)
	for _, service := range titlesResults {
		if !slices.ContainsFunc(results, func(n config.DashboardService) bool { return n.Key == service.Key }) {
			results = append(results, service)
		}
	}
	return results
}
