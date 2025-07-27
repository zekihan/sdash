package dashboard

import (
	"testing"
)

func TestService_GetIconURL(t *testing.T) {
	s := &Service{
		Icon:      "https://example.com/icon.png",
		ProxyIcon: false,
	}
	if got := s.GetIconURL(); got != "https://example.com/icon.png" {
		t.Errorf("GetIconURL() = %v, want %v", got, s.Icon)
	}

	s.ProxyIcon = true
	want := "/api/proxy/icon?url=https%3A%2F%2Fexample.com%2Ficon.png"
	if got := s.GetIconURL(); got != want {
		t.Errorf("GetIconURL() = %v, want %v", got, want)
	}
}

func TestConfig_postProcessServices(t *testing.T) {
	c := &Config{
		Services: map[string]*Service{
			"svc1": {Title: "A", Group: "", Tags: nil, Order: 2},
			"svc2": {Title: "B", Group: "custom", Tags: []string{"tag"}, Order: 1},
		},
		Groups: map[string]*Group{
			"custom": {Key: "custom", Title: "Custom", Order: 1},
		},
		OrderServicesVia: OrderingOrder,
	}

	c.postProcessServices()

	// Check that Key is set
	if c.Services["svc1"].Key != "svc1" {
		t.Errorf("Key not set correctly, got %v", c.Services["svc1"].Key)
	}
	// Check that Tags is not nil
	if c.Services["svc1"].Tags == nil {
		t.Error("Tags should not be nil")
	}
	// Check that default group is set
	if c.Services["svc1"].Group != "others" {
		t.Errorf("Default group not set, got %v", c.Services["svc1"].Group)
	}
	// Check that group is created for default
	if _, ok := c.Groups["others"]; !ok {
		t.Error("Default group 'others' not created")
	}
	// Check that HasAny is set
	if !c.Groups["custom"].HasAny {
		t.Error("Group HasAny not set")
	}
	// Check that ServicesList is sorted by Order
	if len(c.ServicesList) != 2 {
		t.Fatalf("ServicesList length = %d, want 2", len(c.ServicesList))
	}
	if c.ServicesList[0].Order != 1 || c.ServicesList[1].Order != 2 {
		t.Errorf("ServicesList not sorted by Order: %+v", c.ServicesList)
	}
}

func TestServicesList_Sort(t *testing.T) {
	s1 := &Service{Title: "B", Key: "b", Order: 2}
	s2 := &Service{Title: "A", Key: "a", Order: 1}
	list := ServicesList{s1, s2}

	list.Sort(OrderingTitle)
	if list[0].Title != "A" {
		t.Errorf("Sort by title failed: %+v", list)
	}

	list = ServicesList{s1, s2}
	list.Sort(OrderingKey)
	if list[0].Key != "a" {
		t.Errorf("Sort by key failed: %+v", list)
	}

	list = ServicesList{s1, s2}
	list.Sort(OrderingOrder)
	if list[0].Order != 1 {
		t.Errorf("Sort by order failed: %+v", list)
	}

	list = ServicesList{s1, s2}
	list.Sort(OrderingNone)
	// Should not sort, so first is s1
	if list[0] != s1 {
		t.Errorf("Sort with OrderingNone should not sort: %+v", list)
	}
}
