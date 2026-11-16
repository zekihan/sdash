package dashboard

// Settings holds presentation-level dashboard configuration from
// dashboard.yaml. Every field is optional; postProcessSettings fills in the
// defaults so older dashboard.yaml files keep working unchanged.
type Settings struct {
	OpenInNewTab      *bool  `yaml:"openInNewTab"`
	ShowClock         *bool  `yaml:"showClock"`
	ShowWidgetOnCards *bool  `yaml:"showWidgetOnCards"`
	Title             string `yaml:"title"`
	Subtitle          string `yaml:"subtitle"`
	SearchEngine      string `yaml:"searchEngine"`
	SearchEngineName  string `yaml:"searchEngineName"`
	Background        string `yaml:"background"`
	DefaultTheme      string `yaml:"defaultTheme"`
	DefaultAccent     string `yaml:"defaultAccent"`
	DefaultDensity    string `yaml:"defaultDensity"`
}

func (cfg *Config) postProcessSettings() {
	if cfg.Settings == nil {
		cfg.Settings = new(Settings)
	}

	settings := cfg.Settings
	if settings.Title == "" {
		settings.Title = "Homelab"
	}

	if settings.SearchEngine == "" {
		settings.SearchEngine = "https://duckduckgo.com/?q=%s"
		settings.SearchEngineName = "DuckDuckGo"
	}

	switch settings.DefaultTheme {
	case "dark", "light", "system":
	default:
		settings.DefaultTheme = "dark"
	}

	if settings.DefaultAccent == "" {
		settings.DefaultAccent = "lime"
	}

	switch settings.DefaultDensity {
	case "comfortable", "compact":
	default:
		settings.DefaultDensity = "comfortable"
	}

	if settings.OpenInNewTab == nil {
		openInNewTab := true
		settings.OpenInNewTab = &openInNewTab
	}

	if settings.ShowClock == nil {
		showClock := true
		settings.ShowClock = &showClock
	}
}
