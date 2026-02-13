<<<<<<< HEAD
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Instala dependências
RUN apk add --no-cache git

# Copia go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copia código
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o eva-mind ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copia binary
COPY --from=builder /app/eva-mind .

# Expõe portas
EXPOSE 8080 9090

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

CMD ["./eva-mind"]
=======
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

# Copy Python API server (FastAPI bridge)
COPY --from=builder /app/api_server.py ./api_server.py
COPY --from=builder /app/requirements.txt ./requirements.txt

# Cloud Run uses PORT env var
ENV PORT=8091
EXPOSE 8091

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT}/api/health || exit 1

ENTRYPOINT ["./eva-mind"]
>>>>>>> master
