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
				test.query,
				dashboard.GetConfigWithContext(t.Context()),
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

// A request can retain its dashboard after a reload publishes a new one.
// Searching that request must use the retained groups and search settings.
func TestFuzzySearchUsesRequestSnapshot(t *testing.T) {
	current := dashboard.GetConfigWithContext(t.Context())
	service := new(dashboard.Service)
	service.Key = "retained"
	service.Group = "removed-on-reload"
	group := new(dashboard.Group)
	group.Key = service.Group
	group.Title = "SnapshotGroup"
	snapshot := new(dashboard.Config)
	snapshot.ServicesList = dashboard.ServicesList{service}
	snapshot.Groups = map[string]*dashboard.Group{service.Group: group}
	snapshot.SearchConfig = new(dashboard.SearchConfig)
	snapshot.SearchConfig.From = new(dashboard.SearchFromConfig)
	snapshot.SearchConfig.From.Group = []dashboard.Field{dashboard.FieldTitle}

	if current.Groups[service.Group] != nil {
		t.Fatal("fixture group unexpectedly exists in current dashboard")
	}

	results := FuzzySearch("SnapshotGroup", snapshot)
	if len(results) != 1 || results[0] != service {
		t.Fatalf("search lost the request's group metadata: %v", results)
	}
}

func TestFuzzySearchUsesSnapshotSearchSettings(t *testing.T) {
	service := new(dashboard.Service)
	service.Title = "UnsearchedTitle"
	service.Tags = []string{"left", "right"}
	snapshot := new(dashboard.Config)
	snapshot.ServicesList = dashboard.ServicesList{service}
	snapshot.SearchConfig = new(dashboard.SearchConfig)
	snapshot.SearchConfig.From = new(dashboard.SearchFromConfig)
	snapshot.SearchConfig.From.Service = []dashboard.Field{dashboard.FieldTags}
	snapshot.SearchConfig.Separator = "|||"

	if results := FuzzySearch("|||", snapshot); len(results) != 1 || results[0] != service {
		t.Fatalf("search did not use snapshot fields and separator: %v", results)
	}

	if results := FuzzySearch(service.Title, snapshot); len(results) != 0 {
		t.Fatalf("search used a field excluded by the snapshot: %v", results)
	}
}
