FROM golang:1.26-alpine3.24 AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/placidus ./cmd/placidus

FROM alpine:3.24.1

ARG TARGETARCH
RUN apk add --no-cache \
        ca-certificates \
        ffmpeg \
        libva-utils \
        mesa-va-gallium \
        tzdata \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        apk add --no-cache \
            intel-media-driver \
            libva-intel-driver \
            onevpl-intel-gpu; \
    fi
COPY --from=build /out/placidus /usr/local/bin/placidus

ENV PLACIDUS_LISTEN=:40773 \
    PLACIDUS_DATA_DIR=/var/lib/placidus \
    PLACIDUS_RECORDING_DIR=/var/lib/placidus/recordings \
    PLACIDUS_GPU_DEVICE=/dev/dri/renderD128

VOLUME ["/var/lib/placidus"]
EXPOSE 40773
ENTRYPOINT ["/usr/local/bin/placidus"]
