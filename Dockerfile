FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/placidus ./cmd/placidus

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/placidus /usr/local/bin/placidus

ENV PLACIDUS_LISTEN=:40773 \
    PLACIDUS_DATA_DIR=/var/lib/placidus \
    PLACIDUS_RECORDING_DIR=/var/lib/placidus/recordings
VOLUME ["/var/lib/placidus"]
EXPOSE 40773
ENTRYPOINT ["/usr/local/bin/placidus"]
