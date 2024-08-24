package dashboard

import (
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
)

type Service struct {
	Key       string   `yaml:"-"`
	Title     string   `yaml:"title"`
	Group     string   `yaml:"group"`
	Location  string   `yaml:"location"`
	Cluster   string   `yaml:"cluster"`
	URL       string   `yaml:"url"`
	Icon      string   `yaml:"icon"`
	ProxyIcon bool     `yaml:"proxy_icon"`
	Tags      []string `yaml:"tags"`
	Order     int      `yaml:"order"`
}

func (s *Service) GetIconURL() string {
	if !s.ProxyIcon {
		return s.Icon
	}
	return fmt.Sprintf("/icon?url=%s", url.QueryEscape(s.Icon))
}

func (c *Config) postProcessServices() {
	if c.Services == nil {
		c.Services = make(map[string]*Service)
	}
	serviceList := make(ServicesList, 0)
	for k, v := range c.Services {
		v.Key = k
		if v.Tags == nil {
			v.Tags = make([]string, 0)
		}
		if v.Group == "" {
			v.Group = "others"
		}
		if _, ok := c.Groups[v.Group]; ok {
			group := c.Groups[v.Group]
			group.HasAny = true
		} else {
			c.Groups[v.Group] = &Group{
				Key:    v.Group,
				Title:  v.Group,
				Order:  math.MaxInt,
				HasAny: true,
			}
		}
		serviceList = append(serviceList, v)
	}
	serviceList.Sort(c.OrderServicesVia)
	c.ServicesList = serviceList
}

func (c *ServicesList) Sort(ordering Ordering) {
	if ordering != OrderingNone {
		slices.SortStableFunc(*c, func(a, b *Service) int {
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
