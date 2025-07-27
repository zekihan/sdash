package dashboard

import "testing"

func TestGroupListSort(t *testing.T) {
	base := GroupList{
		&Group{Key: "b", Title: "B", Order: 2},
		&Group{Key: "a", Title: "A", Order: 1},
		&Group{Key: "c", Title: "C", Order: 3},
	}

	cases := []struct {
		name     string
		ordering Ordering
		want     []string
	}{
		{"None", OrderingNone, []string{"b", "a", "c"}},
		{"Key", OrderingKey, []string{"a", "b", "c"}},
		{"Title", OrderingTitle, []string{"a", "b", "c"}},
		{"Order", OrderingOrder, []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lst := append(GroupList{}, base...)
			lst.Sort(tc.ordering)
			if len(lst) != len(tc.want) {
				t.Fatalf("got %d items, want %d", len(lst), len(tc.want))
			}
			for i, g := range lst {
				if g.Key != tc.want[i] {
					t.Errorf("at %d: got %q, want %q", i, g.Key, tc.want[i])
				}
			}
		})
	}
}

func TestPostProcessGroups(t *testing.T) {
	t.Run("nil groups yields only others", func(t *testing.T) {
		cfg := &Config{}
		cfg.postProcessGroups()
		if len(cfg.GroupList) != 1 {
			t.Fatalf("expected 1 group, got %d", len(cfg.GroupList))
		}
		if cfg.GroupList[0].Key != "others" {
			t.Errorf("expected key 'others', got %q", cfg.GroupList[0].Key)
		}
	})

	t.Run("prepopulated sort by key + others appended", func(t *testing.T) {
		cfg := &Config{
			Groups: map[string]*Group{
				"b": {Title: "TitleB", Order: 2},
				"a": {Title: "TitleA", Order: 1},
			},
			OrderGroupsVia: OrderingKey,
		}
		cfg.postProcessGroups()
		want := []string{"a", "b", "others"}
		if len(cfg.GroupList) != len(want) {
			t.Fatalf("expected %d groups, got %d", len(want), len(cfg.GroupList))
		}
		for i, g := range cfg.GroupList {
			if g.Key != want[i] {
				t.Errorf("at %d: got %q, want %q", i, g.Key, want[i])
			}
		}
	})
}
