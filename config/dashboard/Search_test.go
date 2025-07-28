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
			input: &Config{},
			expected: &SearchConfig{
				From: &SearchFromConfig{
					Service: []Field{FieldTitle},
					Group:   []Field{FieldTitle},
				},
			},
		},
		{
			name: "nil From",
			input: &Config{
				SearchConfig: &SearchConfig{},
			},
			expected: &SearchConfig{
				From: &SearchFromConfig{
					Service: []Field{FieldTitle},
					Group:   []Field{FieldTitle},
				},
			},
		},
		{
			name: "nil Service and Group",
			input: &Config{
				SearchConfig: &SearchConfig{
					From: &SearchFromConfig{},
				},
			},
			expected: &SearchConfig{
				From: &SearchFromConfig{
					Service: []Field{FieldTitle},
					Group:   []Field{FieldTitle},
				},
			},
		},
		{
			name: "Service and Group already set",
			input: &Config{
				SearchConfig: &SearchConfig{
					From: &SearchFromConfig{
						Service: []Field{FieldKey, FieldTitle},
						Group:   []Field{FieldCluster},
					},
				},
			},
			expected: &SearchConfig{
				From: &SearchFromConfig{
					Service: []Field{FieldKey, FieldTitle},
					Group:   []Field{FieldCluster},
				},
			},
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
