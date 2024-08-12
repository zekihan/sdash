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

func (s Services) String(i int) string {
	item := s[i]
	searchable := []string{item.Title, item.Group, item.Location, item.Cluster}
	searchable = append(searchable, item.Tags...)
	return strings.Join(searchable, "§§§")
}

func (s Services) Len() int {
	return len(s)
}

func FuzzySearch(query string, list Services) Services {
	matches := fuzzy.FindFromNoSort(query, list)
	results := make(Services, len(matches))
	for i, match := range matches {
		results[i] = list[match.Index]
	}
	return results
}
