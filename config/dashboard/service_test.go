package dashboard

import (
	"testing"
)

func TestService_GetIconURL(t *testing.T) {
	service := new(Service)
	service.Icon = "https://example.com/icon.png"
	service.ProxyIcon = false

	if got := service.GetIconURL(); got != "https://example.com/icon.png" {
		t.Errorf("GetIconURL() = %v, want %v", got, service.Icon)
	}

	service.ProxyIcon = true

	want := "/api/proxy/icon?url=https%3A%2F%2Fexample.com%2Ficon.png"
	if got := service.GetIconURL(); got != want {
		t.Errorf("GetIconURL() = %v, want %v", got, want)
	}
}

func TestService_ProbeURL(t *testing.T) {
	service := new(Service)
	service.URL = "https://example.com"

	if got := service.ProbeURL(); got != service.URL {
		t.Errorf("ProbeURL() = %v, want service URL %v", got, service.URL)
	}

	service.HealthURL = "https://example.com/health"
	if got := service.ProbeURL(); got != service.HealthURL {
		t.Errorf("ProbeURL() = %v, want health URL %v", got, service.HealthURL)
	}
}

func TestService_IsHealthyStatus(t *testing.T) {
	service := new(Service)

	for _, code := range []int{200, 204, 301, 302, 401, 403} {
		if !service.IsHealthyStatus(code) {
			t.Errorf("IsHealthyStatus(%d) = false, want true with defaults", code)
		}
	}

	for _, code := range []int{404, 500, 503} {
		if service.IsHealthyStatus(code) {
			t.Errorf("IsHealthyStatus(%d) = true, want false with defaults", code)
		}
	}

	service.HealthStatuses = []int{200, 418}
	if !service.IsHealthyStatus(418) || service.IsHealthyStatus(401) {
		t.Error("explicit healthStatuses must replace the default accepted codes")
	}
}

func TestConfig_postProcessServices(t *testing.T) {
	cfg := new(Config)
	cfg.Services = map[string]*Service{
		"svc1": testService("", "A", "", nil, 2),
		"svc2": testService("", "B", "custom", []string{"tag"}, 1),
	}
	cfg.Groups = map[string]*Group{
		"custom": testGroup("custom", "Custom", 1),
	}
	cfg.OrderServicesVia = OrderingOrder

	cfg.postProcessServices()

	// Check that Key is set
	if cfg.Services["svc1"].Key != "svc1" {
		t.Errorf("Key not set correctly, got %v", cfg.Services["svc1"].Key)
	}
	// Check that Tags is not nil
	if cfg.Services["svc1"].Tags == nil {
		t.Error("Tags should not be nil")
	}
	// Check that default group is set
	if cfg.Services["svc1"].Group != DefaultGroup {
		t.Errorf("Default group not set, got %v", cfg.Services["svc1"].Group)
	}
	// Check that group is created for default
	if _, ok := cfg.Groups[DefaultGroup]; !ok {
		t.Error("Default group 'others' not created")
	}
	// Check that HasAny is set
	if !cfg.Groups["custom"].HasAny {
		t.Error("Group HasAny not set")
	}
	// Check that ServicesList is sorted by Order
	if len(cfg.ServicesList) != 2 {
		t.Fatalf("ServicesList length = %d, want 2", len(cfg.ServicesList))
	}

	if cfg.ServicesList[0].Order != 1 || cfg.ServicesList[1].Order != 2 {
		t.Errorf("ServicesList not sorted by Order: %+v", cfg.ServicesList)
	}
}

func TestServicesList_Sort(t *testing.T) {
	service1 := testService("b", "B", "", nil, 2)
	service2 := testService("a", "A", "", nil, 1)
	list := ServicesList{service1, service2}

	list.Sort(OrderingTitle)

	if list[0].Title != "A" {
		t.Errorf("Sort by title failed: %+v", list)
	}

	list = ServicesList{service1, service2}
	list.Sort(OrderingKey)

	if list[0].Key != "a" {
		t.Errorf("Sort by key failed: %+v", list)
	}

	list = ServicesList{service1, service2}
	list.Sort(OrderingOrder)

	if list[0].Order != 1 {
		t.Errorf("Sort by order failed: %+v", list)
	}

	list = ServicesList{service1, service2}
	list.Sort(OrderingNone)
	// Should not sort, so first is service1
	if list[0] != service1 {
		t.Errorf("Sort with OrderingNone should not sort: %+v", list)
	}
}

func testService(key, title, group string, tags []string, order int) *Service {
	service := new(Service)
	service.Key = key
	service.Title = title
	service.Group = group
	service.Tags = tags
	service.Order = order

	return service
}
