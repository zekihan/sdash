package dashboard

import (
	"slices"
	"strings"
)

type Group struct {
	Key    string `yaml:"-"`
	Title  string `yaml:"title"`
	HasAny bool   `yaml:"-"`
	Order  int    `yaml:"order"`
}

func (cfg *Config) postProcessGroups() {
	if cfg.Groups == nil {
		cfg.Groups = make(map[string]*Group)
	}

	if _, ok := cfg.Groups[DefaultGroup]; !ok {
		cfg.Groups[DefaultGroup] = &Group{Title: "Others"}
	}

	groupList := make(GroupList, 0, len(cfg.Groups))

	for k, v := range cfg.Groups {
		v.Key = k
		groupList = append(groupList, v)
	}

	groupList.Sort(cfg.OrderGroupsVia)
	cfg.GroupList = groupList
}

func (groupList *GroupList) Sort(ordering Ordering) {
	if ordering != OrderingNone {
		slices.SortStableFunc(*groupList, func(firstItem, secondItem *Group) int {
			switch ordering {
			case OrderingTitle:
				return strings.Compare(firstItem.Title, secondItem.Title)
			case OrderingKey:
				return strings.Compare(firstItem.Key, secondItem.Key)
			case OrderingOrder:
				return firstItem.Order - secondItem.Order
			case OrderingNone:
				return 0
			default:
				return 0
			}
		})
	}
}
