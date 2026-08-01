# placidus

placidus is a small, Mirakurun-only web television server written in Go. It provides a server-rendered program guide, live HLS viewing, recording reservations, original MPEG-TS recordings, and explicit tuner/encoder warnings in a quiet YouTube-inspired interface.

This is an early working implementation. Run it on a trusted network: authentication and TLS termination are not included yet.

## Requirements

- Go 1.26 or later when building from source.
- A reachable Mirakurun server with EPG data and at least one configured tuner.
- FFmpeg on `PATH` for live viewing. Recording the original MPEG-TS stream does not require transcoding.
- A modern browser. Safari can play HLS natively; other supported browsers load the pinned HLS.js player from jsDelivr.

placidus probes real encoder initialization at startup. It prefers hardware encoders exposed through FFmpeg (NVENC, QSV, VAAPI, VideoToolbox, or V4L2 M2M) and uses `libx264` only when no hardware path works. Software fallback is always shown in logs and in the web interface.

## Quick Start

Start Mirakurun first, install FFmpeg, and run:

```sh
go run ./cmd/placidus
```

Open <http://localhost:40773>. The default Mirakurun endpoint is <http://127.0.0.1:40772>.

To use a remote Mirakurun server:

```sh
PLACIDUS_MIRAKURUN_URL=http://mirakurun.example:40772 \
PLACIDUS_TUNER_LIMIT=2 \
go run ./cmd/placidus
```

The application still starts when Mirakurun or FFmpeg is unavailable so the home page and `/healthz` can report actionable diagnostics. Live viewing requires FFmpeg; guide and recording operations require Mirakurun.

An Alpine Linux multi-stage image is also provided:

```sh
docker build -t placidus .
docker run --rm -p 40773:40773 \
  -e PLACIDUS_MIRAKURUN_URL=http://mirakurun:40772 \
  -v placidus-data:/var/lib/placidus \
  placidus
```

Pass the relevant GPU or `/dev/dri` device options to the container when hardware encoding is required.

## Configuration

All configuration is read from environment variables.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PLACIDUS_LISTEN` | `:40773` | HTTP listen address |
| `PLACIDUS_MIRAKURUN_URL` | `http://127.0.0.1:40772` | Mirakurun base URL |
| `PLACIDUS_DATA_DIR` | `./data` | Reservation database and temporary HLS root |
| `PLACIDUS_RECORDING_DIR` | `$PLACIDUS_DATA_DIR/recordings` | Original MPEG-TS recording directory |
| `PLACIDUS_FFMPEG` | `ffmpeg` | FFmpeg executable path |
| `PLACIDUS_ENCODER` | `auto` | Preferred FFmpeg hardware encoder name |
| `PLACIDUS_TUNER_LIMIT` | `1` | Number of independently tunable services available to placidus |
| `PLACIDUS_RECORDING_MARGIN` | `0s` | Extra time recorded before and after the EPG window |

`PLACIDUS_TUNER_LIMIT` should match the tuner capacity made available to placidus. Reservations on the same Mirakurun service share capacity. When recordings occupy the configured capacity, unrelated live channels are blocked and the UI explains that only a service already being recorded can be watched.

Example configuration is available in [`.env.example`](.env.example). The program does not load `.env` automatically; export the variables with your process manager or container runtime.

## FFmpeg Hardware Setup

Use `ffmpeg -encoders` to confirm the expected encoder is compiled in, then verify the host exposes its device and driver to the placidus process. Merely appearing in the encoder list is not enough: placidus encodes a short synthetic frame during startup to verify initialization.

Common choices include:

- NVIDIA: `h264_nvenc`
- Intel: `h264_qsv`
- Linux VA-API: `h264_vaapi` with `/dev/dri/renderD128` available
- macOS: `h264_videotoolbox`
- V4L2 devices: `h264_v4l2m2m`

If the preferred encoder fails, placidus tries the remaining hardware candidates before warning and falling back to `libx264`. If a hardware encoder fails while a stream is initializing, the shared live session restarts with `libx264` and the player updates its warning automatically.

## Recording Behavior

Reservations are stored atomically in `reservations.json`. At airtime, placidus opens the Mirakurun service stream with recording priority and writes the original transport stream to an opaque `.ts` filename. Completed files can be played or downloaded from the library screen.

Recordings have priority over ordinary live viewing. State is reconciled after a restart, conflicts are rejected when they are known in advance, and early stream termination is recorded as a visible failure instead of being silently treated as success.

## Development

Run all tests and static checks:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Build without leaving a binary in the repository:

```sh
go build -o /tmp/placidus ./cmd/placidus
```

Frontend templates live under `internal/web/templates` and use the `.gotmpl` extension. All CSS is embedded in the shared template; there are no standalone CSS assets or viewer comment features.

## Health Endpoint

`GET /healthz` returns process and dependency status as JSON:

```json
{"encoder":"h264_nvenc","ffmpeg":true,"mirakurun":true,"status":"running"}
```

The endpoint reports dependency readiness without terminating the web process, which makes initial setup diagnostics available in the browser.
