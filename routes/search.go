package routes

import (
	"fmt"
	"github.com/sahilm/fuzzy"
	"github.com/zekihan/sdash/config"
	"github.com/zekihan/sdash/views"
	"net/http"
	"strings"
)

type Services []config.DashboardService

func GetSearch(w http.ResponseWriter, r *http.Request) {
	dashboard := config.GetDashboardConfig()
	result := dashboard.Services

	query := r.URL.Query().Get("search")
	if query != "" {
		result = FuzzySearch(query, dashboard.Services)
	}
	err := views.CardGrid(&config.DashboardConfig{Services: result}).Render(r.Context(), w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Services) String(i int) string {
	tags := strings.Join(s[i].Tags, "§§§")
	return fmt.Sprintf("%s§§§%s§§§%s", s[i].Name, s[i].Title, tags)
}

func (s Services) Len() int {
	return len(s)
}

func FuzzySearch(query string, list Services) Services {
	// Perform fuzzy matching
	matches := fuzzy.FindFromNoSort(query, list)

	// Extract the matched strings
	results := make(Services, len(matches))
	for i, match := range matches {
		results[i] = list[match.Index]
	}

	return results
}
