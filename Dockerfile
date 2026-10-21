FROM alpine:3.24.1 AS ffmpeg-build

# Alpine's FFmpeg omits NVENC, so build a pinned minimal variant with all placidus encoders.
ARG TARGETARCH
ARG FFMPEG_VERSION=8.1.2
ARG FFMPEG_SHA256=464beb5e7bf0c311e68b45ae2f04e9cc2af88851abb4082231742a74d97b524c
ARG NV_CODEC_HEADERS_VERSION=12.1.14.0
ARG NV_CODEC_HEADERS_SHA256=62b30ab37e4e9be0d0c5b37b8fee4b094e38e570984d56e1135a6b6c2c164c9f
RUN apk add --no-cache \
        build-base \
        ca-certificates \
        coreutils \
        curl \
        libdrm-dev \
        libva-dev \
        nasm \
        openssl-dev \
        pkgconf \
        tar \
        x264-dev \
        xz \
    && if [ "$TARGETARCH" = "amd64" ]; then apk add --no-cache libvpl-dev; fi
RUN curl --retry 5 --retry-all-errors -fsSLo /tmp/nv-codec-headers.tar.gz \
        "https://github.com/FFmpeg/nv-codec-headers/releases/download/n${NV_CODEC_HEADERS_VERSION}/nv-codec-headers-${NV_CODEC_HEADERS_VERSION}.tar.gz" \
    && echo "${NV_CODEC_HEADERS_SHA256}  /tmp/nv-codec-headers.tar.gz" | sha256sum -c - \
    && mkdir /tmp/nv-codec-headers \
    && tar -xzf /tmp/nv-codec-headers.tar.gz -C /tmp/nv-codec-headers --strip-components=1 \
    && make -C /tmp/nv-codec-headers PREFIX=/usr/local install \
    && curl --retry 5 --retry-all-errors -fsSLo /tmp/ffmpeg.tar.xz \
        "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" \
    && echo "${FFMPEG_SHA256}  /tmp/ffmpeg.tar.xz" | sha256sum -c - \
    && mkdir /tmp/ffmpeg \
    && tar -xJf /tmp/ffmpeg.tar.xz -C /tmp/ffmpeg --strip-components=1
RUN cd /tmp/ffmpeg \
    && qsv_flags="" \
    && if [ "$TARGETARCH" = "amd64" ]; then qsv_flags="--enable-libvpl --enable-encoder=h264_qsv"; fi \
    && (./configure \
        --prefix=/usr/local \
        --disable-autodetect \
        --disable-debug \
        --disable-doc \
        --disable-everything \
        --disable-static \
        --enable-avcodec \
        --enable-avfilter \
        --enable-avformat \
        --enable-bsf=aac_adtstoasc,h264_mp4toannexb,hevc_mp4toannexb \
        --enable-decoder=aac,aac_latm,ac3,eac3,h264,hevc,mp2,mp2float,mpeg2video,wrapped_avframe \
        --enable-demuxer=mpegts \
        --enable-encoder=aac,h264_nvenc,h264_v4l2m2m,h264_vaapi,libx264 \
        --enable-ffmpeg \
        --enable-ffnvcodec \
        --enable-ffprobe \
        --enable-filter=color,format,hwupload \
        --enable-gpl \
        --enable-indev=lavfi \
        --enable-libdrm \
        --enable-libx264 \
        --enable-muxer=hls,mp4,mpegts,null \
        --enable-network \
        --enable-nvenc \
        --enable-openssl \
        --enable-parser=aac,aac_latm,ac3,h264,hevc,mpegaudio,mpegvideo \
        --enable-protocol=crypto,file,http,https,tcp,tls,udp \
        --enable-shared \
        --enable-swresample \
        --enable-swscale \
        --enable-vaapi \
        --enable-version3 \
        --extra-cflags=-I/usr/local/include \
        --extra-ldflags=-L/usr/local/lib \
        $qsv_flags || { grep -A12 -B4 nvEncodeAPI.h ffbuild/config.log | tail -80; exit 1; })
RUN make -C /tmp/ffmpeg -j"$(getconf _NPROCESSORS_ONLN)" \
    && make -C /tmp/ffmpeg install

FROM node:24-alpine AS ui-build

WORKDIR /src
RUN npm install --global pnpm@11.24.0
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY index.html tsconfig.json vite.config.ts ./
COPY src ./src
RUN pnpm run build

FROM golang:1.26-alpine3.24 AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=ui-build /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/placidus ./cmd/placidus

FROM alpine:3.24.1

ARG TARGETARCH
# Intel media packages are published for amd64; Mesa VA-API covers supported AMD devices on both image architectures.
RUN apk add --no-cache \
        ca-certificates \
        gcompat \
        libdrm \
        libva \
        libva-utils \
        mesa-va-gallium \
        openssl \
        tzdata \
        x264-libs \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        apk add --no-cache \
            intel-media-driver \
            libvpl \
            libva-intel-driver \
            onevpl-intel-gpu; \
    fi
COPY --from=ffmpeg-build /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg-build /usr/local/bin/ffprobe /usr/local/bin/ffprobe
COPY --from=ffmpeg-build /usr/local/lib/ /usr/local/lib/
COPY --from=build /out/placidus /usr/local/bin/placidus

RUN ffmpeg -hide_banner -encoders > /tmp/encoders.txt 2>&1 \
    && grep -q h264_nvenc /tmp/encoders.txt \
    && grep -q h264_vaapi /tmp/encoders.txt \
    && grep -q libx264 /tmp/encoders.txt \
    && if [ "$TARGETARCH" = "amd64" ]; then grep -q h264_qsv /tmp/encoders.txt; fi \
    && test -e /usr/lib/dri/radeonsi_drv_video.so \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        test -e /usr/lib/dri/iHD_drv_video.so \
        && test -e /usr/lib/dri/i965_drv_video.so \
        && test -e /usr/lib/libmfx-gen.so.1.2; \
    fi \
    && ffmpeg -hide_banner -loglevel error \
        -f lavfi -i color=c=black:s=128x72:d=0.1 \
        -frames:v 1 -an -c:v libx264 -f null - \
    && rm /tmp/encoders.txt

# NVIDIA Container Toolkit uses the capability value when injecting host-matched driver libraries.
ENV PLACIDUS_LISTEN=:40773 \
    PLACIDUS_DATA_DIR=/var/lib/placidus \
    PLACIDUS_RECORDING_DIR=/var/lib/placidus/recordings \
    PLACIDUS_GPU_DEVICE=/dev/dri/renderD128 \
    NVIDIA_DRIVER_CAPABILITIES=video,utility

VOLUME ["/var/lib/placidus"]
EXPOSE 40773
ENTRYPOINT ["/usr/local/bin/placidus"]
