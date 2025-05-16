# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies (git, ca-certificates) so that "go mod download" can fetch private modules (if any) and update go.sum.
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum (if any) so that "go mod download" (and "go mod tidy") can update go.sum.
COPY go.mod go.sum ./

# (Optional) Run "go mod tidy" (if you want to prune or update go.mod) and then "go mod download" (to update go.sum) so that missing dependencies (e.g. golang.org/x/sys/unix, github.com/klauspost/compress/flate, etc.) are added.
RUN go mod tidy && go mod download

# Copy the rest of the application (including .dockerignore so that test files are excluded) so that "go build" compiles only the "real" service logic.
COPY . .

# Build the application (using "go build -v -o smf . so that the entire package is compiled).
RUN go build -v -o smf .

# Final stage (using alpine, copying the binary, and running "./smf")
FROM alpine:latest

WORKDIR /app

# (Optional) Install ca-certificates (and tzdata) so that the container can do TLS (and use time zones) if needed.
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary (from the builder stage) into /app (or /root) so that "CMD ["./smf"]" works.
COPY --from=builder /app/smf .

CMD ["./smf"]
