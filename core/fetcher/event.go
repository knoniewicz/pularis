package fetcher

type EventConfig struct {
	Name            string                 `toml:"name"`
	IsSession       bool                   `toml:"is_session"`
	OnChangedFields []string               `toml:"on_changed_fields"`
	PublishWhen     string                 `toml:"publish_when"`
	Transform       map[string]interface{} `toml:"transform"`
	Disabled        bool                   `toml:"disable"`
}

type Event struct {
	EventCfg    EventConfig
	Body        []byte
	LastPayload map[string]interface{}
}
