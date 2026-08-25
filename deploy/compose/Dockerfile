FROM golang:1.26.6-bookworm AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sentinel ./cmd/sentinel

# Sentinel intentionally uses a small Debian runtime rather than distroless:
# camera diagnostics and recovery need ffprobe/ffmpeg, v4l2-ctl, smartctl and
# restic as explicit bounded command adapters. Production image versions/digests
# are pinned by the release pipeline; this Dockerfile is the build baseline.
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates ffmpeg smartmontools v4l-utils restic \
 && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10001 --home /nonexistent --shell /usr/sbin/nologin sentinel \
 && mkdir -p /var/lib/home-sentinel \
 && chown sentinel:sentinel /var/lib/home-sentinel
COPY --from=build /out/sentinel /usr/local/bin/sentinel
USER sentinel
WORKDIR /var/lib/home-sentinel
ENTRYPOINT ["/usr/local/bin/sentinel"]
CMD ["serve"]
