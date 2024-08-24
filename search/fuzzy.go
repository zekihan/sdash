package search

import (
	"github.com/sahilm/fuzzy"
	"github.com/zekihan/sdash/config/dashboard"
	"strings"
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

	separator := separator()
	for i, service := range list {
		for j, field := range config.SearchConfig.From.Group {
			group := config.Groups[service.Group]
			switch field {
			case dashboard.FieldKey:
				searchable[j][i] = group.Key
			case dashboard.FieldTitle:
				searchable[j][i] = group.Title
			default:
				searchable[j][i] = ""
			}
		}
		padding := len(config.SearchConfig.From.Group)
		for j, field := range config.SearchConfig.From.Service {
			switch field {
			case dashboard.FieldKey:
				searchable[padding+j][i] = service.Key
			case dashboard.FieldTitle:
				searchable[padding+j][i] = service.Title
			case dashboard.FieldLocation:
				searchable[padding+j][i] = service.Location
			case dashboard.FieldCluster:
				searchable[padding+j][i] = service.Cluster
			case dashboard.FieldTags:
				searchable[padding+j][i] = strings.Join(service.Tags, separator)
			default:
				searchable[padding+j][i] = ""
			}
		}
	}
	return searchable
}

func separator() string {
	const defaultSeparator = "§§§"
	separator := dashboard.GetConfig().SearchConfig.Separator
	if separator == "" {
		return defaultSeparator
	}
	if len(separator) < 3 {
		return defaultSeparator
	}
	return separator
}
