# ============================================================
# EVA-Mind - Multi-stage Dockerfile for Google Cloud Run
# ============================================================
# Build: docker build -t eva-mind .
# Run:   docker run -p 8091:8091 --env-file .env eva-mind
# ============================================================

# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w \
    -X main.Version=${VERSION} \
    -X main.GitCommit=${GIT_COMMIT} \
    -X main.BuildTime=${BUILD_TIME}" \
    -o /eva-mind .

# Stage 2: Minimal runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

# Non-root user
RUN adduser -D -u 1000 eva
USER eva

WORKDIR /app

# Copy binary from builder
COPY --from=builder /eva-mind .

# Copy migrations (needed for auto-migration on startup)
COPY --from=builder /app/migrations ./migrations

# NOTE: Python scripts (api_server.py) moved to docs/legacy-python/
# The Go binary handles all API routes natively — no Python runtime needed.

# Cloud Run uses PORT env var
ENV PORT=8091
EXPOSE 8091

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/api/health || exit 1

ENTRYPOINT ["./eva-mind"]
