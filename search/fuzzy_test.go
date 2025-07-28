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
		{"Auth", []string{"authentik"}},
		{"Jelly", []string{"jellyfin"}},
		{"Home", []string{"homeassistant"}},
		{"rackspace", []string{"authentik"}},
		{"media", []string{"jellyfin"}},
		{"automation", []string{"homeassistant"}},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			results := FuzzySearch(test.query, dashboard.GetConfig().ServicesList)
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
