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
	OpenInNewTab   *bool    `yaml:"openInNewTab"`
	Widget         *Widget  `yaml:"widget"`
	HealthKeyword  string   `yaml:"healthKeyword"`
	Key            string   `yaml:"-"`
	Location       string   `yaml:"location"`
	Cluster        string   `yaml:"cluster"`
	URL            string   `yaml:"url"`
	Icon           string   `yaml:"icon"`
	Title          string   `yaml:"title"`
	Description    string   `yaml:"description"`
	Group          string   `yaml:"group"`
	HealthURL      string   `yaml:"healthUrl"`
	HealthStatuses []int    `yaml:"healthStatuses"`
	Tags           []string `yaml:"tags"`
	Order          int      `yaml:"order"`
	ProxyIcon      bool     `yaml:"proxyIcon"`
	HealthDisabled bool     `yaml:"healthDisabled"`
	HealthInsecure bool     `yaml:"healthInsecure"`
}

// Widget optionally attaches live data to a service card. Type selects the
// integration (e.g. "customapi"); options are passed through to the client.
type Widget struct {
	Options map[string]any `yaml:"options"`
	Type    string         `yaml:"type"`
}

// ProbeURL returns the URL the health prober should check for this service.
func (s *Service) ProbeURL() string {
	if s.HealthURL != "" {
		return s.HealthURL
	}

	return s.URL
}

// IsHealthyStatus reports whether an HTTP status counts as healthy. With no
// explicit healthStatuses, redirects, other 2xx/3xx and auth responses count
// as reachable: most homelab apps answer 302→login or 401 when healthy.
func (s *Service) IsHealthyStatus(code int) bool {
	if len(s.HealthStatuses) > 0 {
		return slices.Contains(s.HealthStatuses, code)
	}

	return (code >= 200 && code < 400) || code == 401 || code == 403
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
