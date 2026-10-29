package dashboard

import (
	"math"
	"slices"
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

func TestListsSortOrderSignedRange(t *testing.T) {
	cases := []struct {
		name   string
		orders []int
		want   []string
	}{
		{"negative before max", []int{-1, math.MaxInt}, []string{"a", "b"}},
		{"max before negative", []int{math.MaxInt, -1}, []string{"b", "a"}},
		{"extremes", []int{math.MaxInt, 0, math.MinInt}, []string{"c", "b", "a"}},
		{"negative values", []int{-1, -3, -2}, []string{"b", "c", "a"}},
		{
			"stable ties",
			[]int{math.MaxInt, -1, math.MaxInt, -1, math.MinInt, math.MinInt},
			[]string{"e", "f", "b", "d", "a", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			services := make(ServicesList, 0, len(tc.orders))

			groups := make(GroupList, 0, len(tc.orders))
			for i, order := range tc.orders {
				key := string(rune('a' + i))
				services = append(services, testService(key, "", "", nil, order))
				groups = append(groups, testGroup(key, "", order))
			}

			services.Sort(OrderingOrder)
			groups.Sort(OrderingOrder)

			serviceKeys := make([]string, 0, len(services))
			for _, service := range services {
				serviceKeys = append(serviceKeys, service.Key)
			}

			groupKeys := make([]string, 0, len(groups))
			for _, group := range groups {
				groupKeys = append(groupKeys, group.Key)
			}

			if !slices.Equal(serviceKeys, tc.want) {
				t.Errorf("service keys = %v, want %v", serviceKeys, tc.want)
			}

			if !slices.Equal(groupKeys, tc.want) {
				t.Errorf("group keys = %v, want %v", groupKeys, tc.want)
			}
		})
	}
}

func TestPostProcessConfigOrderGeneratedGroup(t *testing.T) {
	cfg := new(Config)
	cfg.Services = map[string]*Service{
		"service": testService("", "", "generated", nil, 0),
	}
	cfg.Groups = map[string]*Group{
		"negative": testGroup("", "", -1),
	}
	cfg.OrderGroupsVia = OrderingOrder
	cfg.postProcessConfig()

	if cfg.Groups["generated"].Order != math.MaxInt {
		t.Fatalf("generated group order = %d, want MaxInt", cfg.Groups["generated"].Order)
	}

	want := []string{"negative", DefaultGroup, "generated"}

	got := make([]string, 0, len(cfg.GroupList))
	for _, group := range cfg.GroupList {
		got = append(got, group.Key)
	}

	if !slices.Equal(got, want) {
		t.Errorf("group keys = %v, want %v", got, want)
	}
}
