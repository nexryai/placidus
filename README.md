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

The image includes placidus's FFmpeg build plus Intel QSV/VA-API and AMD VA-API user-space runtimes. GPU kernel drivers always come from the host. NVIDIA's user-space driver libraries are injected by NVIDIA Container Toolkit because they must match the host driver.

## Configuration

All configuration is read from environment variables.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PLACIDUS_LISTEN` | `:40773` | HTTP listen address |
| `PLACIDUS_MIRAKURUN_URL` | `http://127.0.0.1:40772` | Mirakurun base URL |
| `PLACIDUS_LOG_LEVEL` | `info` | Structured log verbosity: `debug`, `info`, `warn`, or `error` |
| `PLACIDUS_DATA_DIR` | `./data` | Reservation database and temporary HLS root |
| `PLACIDUS_RECORDING_DIR` | `$PLACIDUS_DATA_DIR/recordings` | Original MPEG-TS recording directory |
| `PLACIDUS_FFMPEG` | `ffmpeg` | FFmpeg executable path |
| `PLACIDUS_ENCODER` | `auto` | Preferred FFmpeg hardware encoder name |
| `PLACIDUS_GPU_DEVICE` | `/dev/dri/renderD128` | DRM render node used by QSV and VA-API |
| `PLACIDUS_TUNER_LIMIT` | `1` | Number of independently tunable services available to placidus |
| `PLACIDUS_RECORDING_MARGIN` | `0s` | Extra time recorded before and after the EPG window |

`PLACIDUS_TUNER_LIMIT` should match the tuner capacity made available to placidus. Reservations on the same Mirakurun service share capacity. When recordings occupy the configured capacity, unrelated live channels are blocked and the UI explains that only a service already being recorded can be watched.

Logs are emitted as JSON on standard output. The default `info` level records startup, reservations, recordings, tuner allocation, live-session lifecycle, dependency failures, and normal page or mutation requests. Use `PLACIDUS_LOG_LEVEL=debug` to also record Mirakurun metadata calls, shared-session reuse, HLS segment requests, health polling, and other high-frequency diagnostics.

Example configuration is available in [`.env.example`](.env.example). The program does not load `.env` automatically; export the variables with your process manager or container runtime.

## FFmpeg Hardware Setup

The Alpine container image contains an FFmpeg build with `h264_nvenc`, `h264_qsv` on amd64, `h264_vaapi`, and `libx264`. It also contains these Linux user-space runtimes:

- Modern Intel VA-API (`intel-media-driver`) and oneVPL (`onevpl-intel-gpu`) on amd64.
- Legacy Intel VA-API (`libva-intel-driver`) on amd64.
- AMD and supported Mesa VA-API drivers (`mesa-va-gallium`).
- `vainfo` for in-container VA-API diagnostics.

Kernel drivers and physical devices cannot be packaged into a portable container image. The host must have a working GPU driver and expose the GPU to the container. Merely appearing in `ffmpeg -encoders` is not enough: placidus encodes a short synthetic frame during startup to verify initialization.

### Intel QSV or VA-API

Expose the render node and optionally select QSV explicitly:

```yaml
services:
  placidus:
    image: ghcr.io/nexryai/placidus:latest
    devices:
      - /dev/dri/renderD128:/dev/dri/renderD128
    environment:
      PLACIDUS_ENCODER: h264_qsv
      PLACIDUS_GPU_DEVICE: /dev/dri/renderD128
```

Use `h264_vaapi` instead when QSV is unavailable. If the GPU is exposed as another render node, update both the device mapping and `PLACIDUS_GPU_DEVICE`. Useful checks are:

```sh
docker compose exec placidus vainfo --display drm --device /dev/dri/renderD128
docker compose exec placidus ffmpeg -hide_banner -encoders
```

The container process must have read/write permission on the render node. Add the host render group with `group_add` when the container runtime does not grant it through the device mapping.

### AMD VA-API

Use the same DRM mapping with `PLACIDUS_ENCODER=h264_vaapi`. The bundled Mesa runtime selects the applicable `radeonsi` or `r600` driver; the host still needs the corresponding `amdgpu` or `radeon` kernel driver.

### NVIDIA NVENC

Install a compatible NVIDIA driver and NVIDIA Container Toolkit on the host. Compose must request the GPU; a `/dev/dri` mapping is not a substitute for NVIDIA runtime injection:

```yaml
services:
  placidus:
    image: ghcr.io/nexryai/placidus:latest
    gpus: all
    environment:
      PLACIDUS_ENCODER: h264_nvenc
      NVIDIA_DRIVER_CAPABILITIES: video,utility
```

The image contains an NVENC-enabled FFmpeg binary, but it intentionally does not contain NVIDIA kernel modules or version-coupled driver libraries. NVIDIA Container Toolkit mounts the matching host libraries, including the Video Codec SDK runtime, into the container.

Common choices include:

- NVIDIA: `h264_nvenc` with NVIDIA Container Toolkit and `gpus: all`
- Intel: `h264_qsv` with the configured DRM render node available
- Intel or AMD VA-API: `h264_vaapi` with the configured DRM render node available
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
