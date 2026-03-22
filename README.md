# Pularis

Pularis is a small Go service that polls external HTTP APIs, transforms responses into event payloads, and publishes those events to Redis channels.

It is designed for source adapters that should stay stateless: fetch data, map fields, detect session transitions, and emit events. It does not own downstream storage.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## What it does

- Polls HTTP endpoints on an interval
- Maps API responses into event payloads with `gjson` paths
- Supports conditional publishing with `expr`
- Detects session start/end transitions
- Publishes JSON payloads to Redis channels named `pularis.<event>.<action>`

## Repository layout

- `core/` - Go service entrypoint and runtime
- `examples/configs/` - example source config and auth template
- `schemas/` - reserved for shared event/config schemas

## Quick start

1. Start Redis.
2. Copy the example configs into `core/configs/`.
3. Fill in any auth placeholders you need.
4. Run the service from `core/`.

```bash
mkdir -p core/configs
cp examples/configs/example-source.toml core/configs/
cp examples/configs/example-source.auth.json.example core/configs/example-source.auth.json
cd core
go run .
```

Required environment variables:

- `REDIS_ADDR` - Redis host and port, for example `localhost:6379`

Optional environment variables:

- `CONFIGS_DIR` - fetcher config directory, defaults to `configs`
- `MAX_WORKERS` - worker pool size, defaults to CPU count

## Run with Docker Compose

The repository includes a `docker-compose.yml` that starts Redis and Pularis together.

```bash
docker compose up --build
```

By default the Compose setup mounts `examples/configs/` into the container as `/app/configs`.

If you want to run your own local source definitions instead, change the volume mapping in `docker-compose.yml` to point at your config directory.

## Config example

```toml
name = "example_source"
type = "api"
endpoint = "https://api.example.com/status"
poll_interval_seconds = 15
auth_file = "example-source"

[headers]
Authorization = "Bearer {token}"

[[events]]
name = "activity"
is_session = true
on_changed_fields = ["state", "user_id"]
publish_when = "state != ''"

[events.transform]
user_id = "data.user.id"
state = "data.session.state"
title = "data.session.title"
```

Auth files live beside the TOML configs and use the `.auth.json` suffix:

```json
{
  "token": "replace-me"
}
```

Header values can reference auth keys using `{key}` placeholders.

## Published events

Every publish sends a JSON object like:

```json
{
  "request_id": "uuid",
  "payload": {
    "source": "example_source",
    "id": "session-uuid",
    "started_at": "2026-03-22T00:00:00Z"
  }
}
```

Session events publish to:

- `pularis.<event>.created`
- `pularis.<event>.ended`

Metric-style events publish to:

- `pularis.<event>.created`

## Development

```bash
cd core
go fmt ./...
go test ./...
go build ./...
```

## Project status

- Language: Go
- Module path: `github.com/knoniewicz/pularis`
- License: MIT
- Runtime dependency: Redis

## Contributing

Issues and pull requests are welcome. Keep source integrations generic and stateless so the project stays focused on polling, transforming, and publishing.
