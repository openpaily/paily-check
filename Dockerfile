# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Download dependencies first (cached layer unless go.mod/go.sum change).
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/checker ./cmd/checker

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates is needed for HTTPS pings and external API calls.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Keep the auto-refreshed MMDB in a dedicated writable directory so the file
# can be downloaded on startup and updated in-place every 12 hours.
RUN mkdir -p /app/data

COPY --from=builder /out/checker /app/checker

# Third-party scripts are supplied through a read-only volume at deploy time.
COPY scripts/   /app/scripts/

# Configuration must be mounted at /app/config.yaml. The MMDB is stored in the
# writable /app/data volume and is refreshed there when GEO_MODE=mmdb.
ENV GEO_MMDB_PATH=/app/data/Country-without-asn.mmdb

ENTRYPOINT ["/app/checker", "--config", "/app/config.yaml"]
