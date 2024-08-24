package routes

import (
	"github.com/sahilm/fuzzy"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/views"
	"net/http"
	"slices"
	"strings"
)

func GetSearch(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfig()
	result := config.ServicesList
	groups := config.GroupList

	query := r.URL.Query().Get("search")
	if query != "" {
		result = FuzzySearch(query, result)
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
	if err != nil {
		log.Error("Error rendering dashboard: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("500 - Something bad happened!"))
		if err != nil {
			return
		}
	}
}

func FuzzySearch(query string, list dashboard.ServicesList) dashboard.ServicesList {
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
	results := make([]*dashboard.Service, len(resultsMap))
	i := 0
	for key := range resultsMap {
		results[i] = list[key]
		i++
	}
	return results
}
