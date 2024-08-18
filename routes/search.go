package routes

import (
	"github.com/sahilm/fuzzy"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
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
	resultsMap := make(map[int]bool)
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
	for _, match := range titlesMatches {
		resultsMap[match.Index] = true
	}
	groupsMatches := fuzzy.FindNoSort(query, groups)
	for _, match := range groupsMatches {
		resultsMap[match.Index] = true
	}
	locationsMatches := fuzzy.FindNoSort(query, locations)
	for _, match := range locationsMatches {
		resultsMap[match.Index] = true
	}
	clustersMatches := fuzzy.FindNoSort(query, clusters)
	for _, match := range clustersMatches {
		resultsMap[match.Index] = true
	}
	tagsMatches := fuzzy.FindNoSort(query, tags)
	for _, match := range tagsMatches {
		resultsMap[match.Index] = true
	}
	results := make([]config.DashboardService, len(resultsMap))
	i := 0
	for key, _ := range resultsMap {
		results[i] = list[key]
		i++
	}
	return results
}
