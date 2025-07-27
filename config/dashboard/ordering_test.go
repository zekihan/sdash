package dashboard

import (
	"testing"
)

func TestOrderingConstants(t *testing.T) {
	tests := []struct {
		name     string
		ordering Ordering
		expected string
	}{
		{"None", OrderingNone, ""},
		{"Order", OrderingOrder, "order"},
		{"Title", OrderingTitle, "title"},
		{"Key", OrderingKey, "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.ordering) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.ordering)
			}
		})
	}
}
