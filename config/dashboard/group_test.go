package dashboard

import "testing"

func TestGroupListSort(t *testing.T) {
	base := GroupList{
		testGroup("b", "B", 2),
		testGroup("a", "A", 1),
		testGroup("c", "C", 3),
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
		cfg := new(Config)
		cfg.postProcessGroups()

		if len(cfg.GroupList) != 1 {
			t.Fatalf("expected 1 group, got %d", len(cfg.GroupList))
		}

		if cfg.GroupList[0].Key != DefaultGroup {
			t.Errorf("expected key 'others', got %q", cfg.GroupList[0].Key)
		}
	})

	t.Run("prepopulated sort by key + others appended", func(t *testing.T) {
		cfg := new(Config)
		cfg.Groups = map[string]*Group{
			"b": testGroup("", "TitleB", 2),
			"a": testGroup("", "TitleA", 1),
		}
		cfg.OrderGroupsVia = OrderingKey
		cfg.postProcessGroups()

		want := []string{"a", "b", DefaultGroup}
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

func testGroup(key, title string, order int) *Group {
	group := new(Group)
	group.Key = key
	group.Title = title
	group.Order = order

	return group
}
