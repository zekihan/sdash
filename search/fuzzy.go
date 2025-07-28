package search

import (
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/zekihan/sdash/config/dashboard"
)

const (
	defaultSeparator   = "§§§"
	minSeparatorLength = 3
)

func FuzzySearch(query string, list dashboard.ServicesList) dashboard.ServicesList {
	resultsMap := make(map[int]bool)

	searchable := getSearchable(list)
	for i := range searchable {
		matches := fuzzy.FindNoSort(query, searchable[i])
		for _, match := range matches {
			resultsMap[match.Index] = true
		}
	}

	results := make([]*dashboard.Service, len(resultsMap))

	i := 0
	for key := range resultsMap {
		results[i] = list[key]
		i++
	}

	return results
}

func getSearchable(list dashboard.ServicesList) [][]string {
	config := dashboard.GetConfig()

	size := len(config.SearchConfig.From.Group) + len(config.SearchConfig.From.Service)

	searchable := make([][]string, size)
	for i := range searchable {
		searchable[i] = make([]string, len(list))
	}

	for i, service := range list {
		for j, field := range config.SearchConfig.From.Group {
			group := config.Groups[service.Group]
			searchable[j][i] = getSearchableFromGroup(field, group)
		}

		padding := len(config.SearchConfig.From.Group)
		for j, field := range config.SearchConfig.From.Service {
			searchable[padding+j][i] = GetSearchableFromService(field, service)
		}
	}

	return searchable
}

func getSearchableFromGroup(field dashboard.Field, group *dashboard.Group) string {
	switch field {
	case dashboard.FieldKey:
		return group.Key
	case dashboard.FieldTitle:
		return group.Title
	case dashboard.FieldLocation:
		return ""
	case dashboard.FieldCluster:
		return ""
	case dashboard.FieldTags:
		return ""
	default:
		return ""
	}
}

func GetSearchableFromService(field dashboard.Field, service *dashboard.Service) string {
	separator := getSeparator()

	switch field {
	case dashboard.FieldKey:
		return service.Key
	case dashboard.FieldTitle:
		return service.Title
	case dashboard.FieldLocation:
		return service.Location
	case dashboard.FieldCluster:
		return service.Cluster
	case dashboard.FieldTags:
		return strings.Join(service.Tags, separator)
	default:
		return ""
	}
}

func getSeparator() string {
	separator := dashboard.GetConfig().SearchConfig.Separator
	if separator == "" {
		return defaultSeparator
	}

	if len(separator) < minSeparatorLength {
		return defaultSeparator
	}

	return separator
}
