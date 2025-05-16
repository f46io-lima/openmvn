# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' openmv

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/smf /app/smf
COPY --from=builder /build/configs/smf/config.yaml /app/config.yaml

# Set ownership
RUN chown -R openmv:openmv /app

# Switch to non-root user
USER openmv

# Expose ports
EXPOSE 8805/udp  # GTP-C
EXPOSE 8806/udp  # PFCP
EXPOSE 9090      # Metrics

# Run the service
ENTRYPOINT ["/app/smf"]
CMD ["--config", "/app/config.yaml"] 