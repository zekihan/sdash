package dashboard

import (
	"reflect"
	"testing"
)

func TestPostProcessSearchConfig(t *testing.T) {
	tests := []struct {
		input    *Config
		expected *SearchConfig
		name     string
	}{
		{
			name:  "nil SearchConfig",
			input: testConfig(nil),
			expected: testSearchConfig(
				testSearchFromConfig([]Field{FieldTitle}, []Field{FieldTitle}),
			),
		},
		{
			name:  "nil From",
			input: testConfig(testSearchConfig(nil)),
			expected: testSearchConfig(
				testSearchFromConfig([]Field{FieldTitle}, []Field{FieldTitle}),
			),
		},
		{
			name:  "nil Service and Group",
			input: testConfig(testSearchConfig(testSearchFromConfig(nil, nil))),
			expected: testSearchConfig(
				testSearchFromConfig([]Field{FieldTitle}, []Field{FieldTitle}),
			),
		},
		{
			name: "Service and Group already set",
			input: testConfig(testSearchConfig(testSearchFromConfig(
				[]Field{FieldKey, FieldTitle}, []Field{FieldCluster},
			))),
			expected: testSearchConfig(testSearchFromConfig(
				[]Field{FieldKey, FieldTitle}, []Field{FieldCluster},
			)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.postProcessSearchConfig()

			if tt.input.SearchConfig == nil {
				t.Fatalf("SearchConfig is nil")
			}

			if tt.input.SearchConfig.From == nil {
				t.Fatalf("From is nil")
			}

			if !reflect.DeepEqual(tt.input.SearchConfig.From.Service, tt.expected.From.Service) {
				t.Errorf(
					"Service = %v, want %v",
					tt.input.SearchConfig.From.Service,
					tt.expected.From.Service,
				)
			}

			if !reflect.DeepEqual(tt.input.SearchConfig.From.Group, tt.expected.From.Group) {
				t.Errorf(
					"Group = %v, want %v",
					tt.input.SearchConfig.From.Group,
					tt.expected.From.Group,
				)
			}
		})
	}
}

func testConfig(searchConfig *SearchConfig) *Config {
	cfg := new(Config)
	cfg.SearchConfig = searchConfig

	return cfg
}

func testSearchConfig(from *SearchFromConfig) *SearchConfig {
	cfg := new(SearchConfig)
	cfg.From = from

	return cfg
}

func testSearchFromConfig(service, group []Field) *SearchFromConfig {
	cfg := new(SearchFromConfig)
	cfg.Service = service
	cfg.Group = group

	return cfg
}
