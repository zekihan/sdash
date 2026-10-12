package search

import (
	"testing"

	"github.com/zekihan/sdash/config/dashboard"
)

func TestFuzzySearch(t *testing.T) {
	tests := []struct {
		query    string
		expected []string // Expected service keys
	}{
		{"Docs", []string{"documentation"}},
		{"File", []string{"files"}},
		{"Metrics", []string{"metrics"}},
		{"example", []string{"documentation"}},
		{"demo", []string{"documentation"}},
		{"Monitoring", []string{"metrics"}},
		{"knowledge", []string{"documentation"}},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			results := FuzzySearch(
				t.Context(),
				test.query,
				dashboard.GetConfigWithContext(t.Context()).ServicesList,
			)
			for _, expectedKey := range test.expected {
				found := false

				for _, result := range results {
					if result.Key == expectedKey {
						found = true

						break
					}
				}

				if !found {
					t.Errorf("expected service with key %s not found in results", expectedKey)
				}
			}
		})
	}
}
