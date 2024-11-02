# # Build stage
FROM golang:1.23.2-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o logforwarder .

# Final stage
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    sqlite-libs && \
    mkdir -p /app/cfg /app/logs /app/db

# Copy the binary from builder
COPY --from=builder /build/logforwarder /app/

# Create non-root user
RUN adduser -D -H -h /app appuser && \
    chown -R appuser:appuser /app

USER appuser

# Command to run the application
ENTRYPOINT ["/app/logforwarder"]
