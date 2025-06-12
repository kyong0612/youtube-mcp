# Build stage
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates for HTTPS requests
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Verify dependencies
RUN go mod verify

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags for smaller binary and version injection
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=1.0.0 -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -a -installsuffix cgo \
    -o youtube-transcript-mcp \
    ./cmd/server/main.go

# Final stage - minimal image
FROM alpine:latest

# Install ca-certificates for HTTPS and tzdata for timezones
RUN apk --no-cache add ca-certificates tzdata curl

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set timezone
ENV TZ=UTC

# Create app directory and user
RUN addgroup -g 1000 appuser && \
    adduser -D -H -u 1000 -G appuser appuser && \
    mkdir -p /app && chown appuser:appuser /app

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appuser /build/youtube-transcript-mcp .

# Create directories for logs and data
RUN mkdir -p /app/logs /app/data && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Set default environment variables
ENV PORT=8080 \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    CACHE_ENABLED=true \
    CACHE_TYPE=memory

# Run the application
ENTRYPOINT ["./youtube-transcript-mcp"]