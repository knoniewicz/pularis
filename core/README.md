# pularis

Pularis is a generic poller/publisher service.
It polls external APIs on intervals, transforms responses into event payloads,
and publishes events to Redis.

Pularis does not write to a database and should not carry source-specific logic.

## Responsibilities

- Poll configured API endpoints
- Transform API responses using JSONPath mappings
- Detect session lifecycle transitions (`.created`, `.ended`)
- Publish events on Redis channels (`pularis.<event_name>`)

## Runtime

- Language: Go
- Entry point: `core/main.go`
- Configuration: environment variables + TOML source configs
- Redis required: `REDIS_ADDR`
- Config directory: `CONFIGS_DIR` or `core/configs`

## Config shape

```toml
name = "example_source"
type = "api"
endpoint = "https://api.example.com/status"
poll_interval_seconds = 5

[[events]]
name = "activity"
transform = { field1 = "data.field1", field2 = "data.field2" }
is_session = true
on_changed_fields = ["field1", "field2"]
publish_when = "field1 != ''"
```

## Event lifecycle

- `<event>.created`: emitted when a new session starts or session payload changes
- `<event>.ended`: emitted when an active session ends

## Integration

- Pularis only fetches, transforms, and publishes
