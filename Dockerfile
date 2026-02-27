# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o orchestrator \
    main.go

# Runtime stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 orchestrator && \
    adduser -D -u 1000 -G orchestrator orchestrator

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/orchestrator .
COPY --from=builder /build/configs ./configs
COPY --from=builder /build/static ./static

# Set ownership
RUN chown -R orchestrator:orchestrator /app

USER orchestrator

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/orchestrator"]