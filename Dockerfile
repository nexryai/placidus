FROM golang:1.26-alpine3.24 AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/placidus ./cmd/placidus

FROM alpine:3.24.1

RUN apk add --no-cache ca-certificates ffmpeg tzdata
COPY --from=build /out/placidus /usr/local/bin/placidus

ENV PLACIDUS_LISTEN=:40773 \
    PLACIDUS_DATA_DIR=/var/lib/placidus \
    PLACIDUS_RECORDING_DIR=/var/lib/placidus/recordings

VOLUME ["/var/lib/placidus"]
EXPOSE 40773
ENTRYPOINT ["/usr/local/bin/placidus"]
