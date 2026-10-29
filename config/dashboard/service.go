package dashboard

import (
	"cmp"
	"math"
	"net/url"
	"slices"
	"strings"
)

const DefaultGroup = "others"

type Service struct {
	Key       string   `yaml:"-"`
	Title     string   `yaml:"title"`
	Group     string   `yaml:"group"`
	Location  string   `yaml:"location"`
	Cluster   string   `yaml:"cluster"`
	URL       string   `yaml:"url"`
	Icon      string   `yaml:"icon"`
	Tags      []string `yaml:"tags"`
	Order     int      `yaml:"order"`
	ProxyIcon bool     `yaml:"proxyIcon"`
}

func (s *Service) GetIconURL() string {
	if !s.ProxyIcon {
		return s.Icon
	}

	return "/api/proxy/icon?url=" + url.QueryEscape(s.Icon)
}

func (cfg *Config) postProcessServices() {
	if cfg.Services == nil {
		cfg.Services = make(map[string]*Service)
	}

	serviceList := make(ServicesList, 0, len(cfg.Services))

	for key, value := range cfg.Services {
		value.Key = key
		if value.Tags == nil {
			value.Tags = make([]string, 0)
		}

		if value.Group == "" {
			value.Group = DefaultGroup
		}

		if _, ok := cfg.Groups[value.Group]; ok {
			group := cfg.Groups[value.Group]
			group.HasAny = true
		} else {
			cfg.Groups[value.Group] = &Group{
				Key:    value.Group,
				Title:  value.Group,
				Order:  math.MaxInt,
				HasAny: true,
			}
		}

		serviceList = append(serviceList, value)
	}

	serviceList.Sort(cfg.OrderServicesVia)
	cfg.ServicesList = serviceList
}

func (servicesList *ServicesList) Sort(ordering Ordering) {
	if ordering != OrderingNone {
		slices.SortStableFunc(*servicesList, func(firstItem, secondItem *Service) int {
			switch ordering {
			case OrderingTitle:
				return strings.Compare(firstItem.Title, secondItem.Title)
			case OrderingKey:
				return strings.Compare(firstItem.Key, secondItem.Key)
			case OrderingOrder:
				return cmp.Compare(firstItem.Order, secondItem.Order)
			case OrderingNone:
				return 0
			default:
				return 0
			}
		})
	}
}
