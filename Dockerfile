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
