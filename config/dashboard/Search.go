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
	From *SearchFromConfig `yaml:"from"`
}

func (c *Config) postProcessSearchConfig() {
	if c.SearchConfig == nil {
		c.SearchConfig = &SearchConfig{}
	}
	if c.SearchConfig.From == nil {
		c.SearchConfig.From = &SearchFromConfig{}
	}
	if c.SearchConfig.From.Service == nil {
		c.SearchConfig.From.Service = []Field{FieldTitle}
	}
	if c.SearchConfig.From.Group == nil {
		c.SearchConfig.From.Group = []Field{FieldTitle}
	}
}
