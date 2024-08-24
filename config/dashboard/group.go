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

func (c *Config) postProcessGroups() {
	if c.Groups == nil {
		c.Groups = make(map[string]*Group)
	}
	if _, ok := c.Groups["others"]; !ok {
		c.Groups["others"] = &Group{Title: "Others"}
	}
	groupList := make(GroupList, 0)
	for k, v := range c.Groups {
		v.Key = k
		groupList = append(groupList, v)
	}
	groupList.Sort(c.OrderGroupsVia)
	c.GroupList = groupList
}

func (c *GroupList) Sort(ordering Ordering) {
	if ordering != OrderingNone {
		slices.SortStableFunc(*c, func(a, b *Group) int {
			switch ordering {
			case OrderingTitle:
				return strings.Compare(a.Title, b.Title)
			case OrderingKey:
				return strings.Compare(a.Key, b.Key)
			case OrderingOrder:
				return a.Order - b.Order
			}
			return 0
		})
	}
}
