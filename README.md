# IAS Media Server

ONVIF camera media server. Stores camera credentials in SQLite, streams camera feeds via HLS using ffmpeg.

Part of the **IAS Ecosystem** — developed by [haziqnorisham](https://github.com/haziqnorisham) for **Camart Sdn. Bhd.**

## Prerequisites

- **Go 1.22+**
- **ffmpeg** (must be in `$PATH`)

```sh
# macOS
brew install ffmpeg

# Debian/Ubuntu
sudo apt install ffmpeg
```

## Quick start

```sh
go build -trimpath -o go_onvif main.go
PORT=8080 ./go_onvif
```

The server creates `onvif.db` (SQLite) and `hls/` (HLS segments) on first run.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/devices` | Add camera `{name, ip, port, username, password}` |
| `GET` | `/api/devices` | List all devices |
| `GET` | `/api/devices/{id}` | Get device |
| `PUT` | `/api/devices/{id}` | Update device |
| `DELETE` | `/api/devices/{id}` | Remove device |
| `GET` | `/api/devices/{id}/profiles` | Live ONVIF profiles + RTSP URLs |
| `PUT` | `/api/devices/{id}/stream-profile` | Set profile token for HLS `{token}` |
| `GET` | `/api/streams` | List active streams |
| `POST` | `/api/streams/{device_id}/start` | Start HLS streaming |
| `POST` | `/api/streams/{device_id}/stop` | Stop HLS streaming |
| `GET` | `/hls/{device_id}/index.m3u8` | HLS playlist |

Passwords are never returned in API responses (excluded via `json:"-"`).

## Usage flow

```
1. POST /api/devices          → add camera credentials
2. GET  /api/devices/1/profiles → browse available stream profiles
3. PUT  /api/devices/1/stream-profile → select which profile to stream
4. POST /api/streams/1/start  → start ffmpeg transcoding to HLS
5. Open /hls/1/index.m3u8     → play the stream
```

On startup, all devices with a `stream_profile_token` automatically begin streaming. If ffmpeg crashes, it restarts after 2 seconds.

## OpenAPI

Import `openapi.yaml` into [Bruno](https://www.usebruno.com/) or any OpenAPI client.
