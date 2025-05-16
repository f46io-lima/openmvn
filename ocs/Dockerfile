# Build stage
FROM golang:1.21-alpine as builder

WORKDIR /app

# Install git and certificates
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod tidy && go mod download

# Copy the rest of the application
COPY . .

# Build the entire package
RUN go build -v -o ocs .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/ocs .

EXPOSE 8084

CMD ["./ocs"] 