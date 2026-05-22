# AGENTS.md

## Build & run

```sh
go build -trimpath -o go_onvif main.go
PORT=8080 ./go_onvif
```

Copy `.env.example` to `.env` to set config. `.env` is gitignored (contains camera passwords).

## Prerequisites

- **Go 1.22+** — required for the `"METHOD /path"` mux routing syntax.
- **ffmpeg** in `$PATH` — checked at startup by `stream.NewStreamManager`. Not optional.

## Architecture

Single-binary HTTP server. No frameworks — stdlib `net/http` with Go 1.22 `ServeMux` patterns.

| Directory | Purpose |
|---|---|
| `main.go` | Entrypoint: config, DB init, routes, signal handling, auto-ingest |
| `config/` | Env var loading (PORT, DB_PATH, HLS_DIR, HLS_TIME, HLS_LIST_SIZE) |
| `db/` | SQLite via `modernc.org/sqlite` (pure Go, no CGO). Schema auto-migrates. |
| `handler/` | HTTP handlers — `DeviceHandler` and `StreamHandler` |
| `onvif/` | ONVIF client wrapper around `github.com/gowvp/onvif` |
| `stream/` | ffmpeg HLS stream manager |
| `middleware/` | CORS (permissive: `*` origin, all methods) |
| `hls/` | HLS segments output dir (gitignored, created at runtime) |

## Database (SQLite)

- Driver: `modernc.org/sqlite` — pure Go, **requires `SetMaxOpenConns(1)`** (already set in `db/sqlite.go`).
- File: `onvif.db` by default (gitignored). Auto-created with schema on first run.
- Column `password` is stored in the DB but excluded from all JSON responses via `json:"-"` on the `Device` struct.

## Streaming (ffmpeg)

- Transcodes RTSP to HLS: `libx264`, `ultrafast` preset, `zerolatency` tune, **no audio** (`-an`).
- ffmpeg crashes auto-restart after a 2-second delay.
- On startup, all devices with a `stream_profile_token` set automatically begin streaming (see `autoIngest` in `main.go`).
- Changing a device's stream profile (`PUT /api/devices/{id}/stream-profile`) stops the old stream and starts a new one immediately.

## Testing

No tests exist. No CI/CD is configured.
