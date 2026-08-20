# Multi-stage build
FROM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bitacora-plantas .

# Final runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests and curl for health checks
RUN apk update && apk add --no-cache ca-certificates curl

# Create app directory and user
RUN adduser -D -s /bin/sh appuser
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bitacora-plantas .

# Copy templates and static files
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Create data directory with proper permissions
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/ || exit 1

# Set environment variable for data directory
ENV DATA_DIR=/app/data

# Run the application as root to avoid permission issues with volumes
CMD ["./bitacora-plantas"]