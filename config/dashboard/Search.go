package dashboard

type Field string

const (
	FieldKey      Field = "key"
	FieldTitle    Field = "title"
	FieldLocation Field = "location"
	FieldCluster  Field = "cluster"
	FieldTags     Field = "tags"
)

type SearchFromConfig struct {
	Service []Field `yaml:"service"`
	Group   []Field `yaml:"group"`
}

type SearchConfig struct {
	From      *SearchFromConfig `yaml:"from"`
	Separator string            `yaml:"separator"`
}

func (cfg *Config) postProcessSearchConfig() {
	if cfg.SearchConfig == nil {
		cfg.SearchConfig = &SearchConfig{}
	}

	if cfg.SearchConfig.From == nil {
		cfg.SearchConfig.From = &SearchFromConfig{}
	}

	if cfg.SearchConfig.From.Service == nil {
		cfg.SearchConfig.From.Service = []Field{FieldTitle}
	}

	if cfg.SearchConfig.From.Group == nil {
		cfg.SearchConfig.From.Group = []Field{FieldTitle}
	}
}
