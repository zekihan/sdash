package views

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zekihan/sdash/config/dashboard"
)

func TestCardExternalLink(t *testing.T) {
	var (
		rendered bytes.Buffer
		service  dashboard.Service
	)

	service.Key = "example"
	service.Title = "Example"
	service.URL = "https://example.com/service?one=1&two=2"

	err := card(&service).Render(t.Context(), &rendered)
	if err != nil {
		t.Fatal(err)
	}

	html := rendered.String()
	for _, want := range []string{
		`<a `,
		`href="https://example.com/service?one=1&amp;two=2"`,
		`target="_blank"`,
		`rel="noopener noreferrer"`,
		`Example`,
		`</a>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered card missing %q: %s", want, html)
		}
	}

	for _, unsafe := range []string{"onclick=", "onauxclick=", "window.open"} {
		if strings.Contains(html, unsafe) {
			t.Errorf("rendered card contains obsolete navigation handler %q", unsafe)
		}
	}
}

func TestCardRejectsJavaScriptURL(t *testing.T) {
	var (
		rendered bytes.Buffer
		service  dashboard.Service
	)

	service.Key = "example"
	service.Title = "Example"
	service.URL = "javascript:alert(document.domain)"

	err := card(&service).Render(t.Context(), &rendered)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(rendered.String(), "javascript:") {
		t.Fatal("card rendered an executable JavaScript URL")
	}
}
