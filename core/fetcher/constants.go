package fetcher

const (
	ActionCreated = "created"
	ActionEnded   = "ended"
)

const (
	FieldSource    = "source"
	FieldID        = "id"
	FieldStartedAt = "started_at"
	FieldEndedAt   = "ended_at"
	FieldRequestID = "request_id"
	FieldPayload   = "payload"
)

const (
	TypeAPI = "api"
)

const (
	RedisChannelPrefix = "pularis."
)

const (
	ConfigsDir     = "configs"
	AuthFileSuffix = ".auth.json"
)

const (
	PlaceholderStart = "{"
	PlaceholderEnd   = "}"
)
