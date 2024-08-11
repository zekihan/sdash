package routes

import (
	"github.com/sahilm/fuzzy"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
	"sort"
	"strings"
)

type Services map[string]config.DashboardService

func GetSearch(w http.ResponseWriter, r *http.Request) {
	dashboard := config.GetDashboardConfig()
	result := dashboard.Services

	query := r.URL.Query().Get("search")
	if query != "" {
		result = FuzzySearch(query, dashboard.Services)
	}
	err := views.CardGrid(&config.DashboardConfig{Services: result}).Render(r.Context(), w)
	if err != nil {
		log.Error("Error rendering dashboard: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("500 - Something bad happened!"))
		if err != nil {
			return
		}
	}
}

func (s Services) Get(i int) config.DashboardService {
	return s[s.GetKey(i)]
}

func (s Services) GetKey(i int) string {
	keys := make([]string, 0)
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if i == 0 {
			return k
		}
		i--
	}
	return ""
}

func (s Services) String(i int) string {
	item := s.Get(i)
	searchables := []string{item.Title, item.Group, item.Location, item.Cluster}
	searchables = append(searchables, item.Tags...)
	return strings.Join(searchables, "§§§")
}

func (s Services) Len() int {
	return len(s)
}

func FuzzySearch(query string, list Services) Services {
	// Perform fuzzy matching
	matches := fuzzy.FindFromNoSort(query, list)

	// Extract the matched strings
	results := make(Services, len(matches))
	for _, match := range matches {
		key := list.GetKey(match.Index)
		results[key] = list[key]
	}

	return results
}
