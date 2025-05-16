#!/bin/bash
set -e

SERVICES=(amf smf ocs bss udm upf)
PORTS=(8081 8082 8084 8083 8085 8805)

for i in "${!SERVICES[@]}"; do
  svc="${SERVICES[$i]}"
  port="${PORTS[$i]}"
  echo "🔧 Updating $svc/Dockerfile..."
  
  # Create new Dockerfile content
  cat > "$svc/Dockerfile" << EOF
FROM golang:1.21-alpine as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy && go mod download

COPY . .
RUN go build -v -o $svc main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/$svc .

# Special case for UPF which needs UDP ports
EOF

  # Add appropriate EXPOSE line based on service
  if [ "$svc" = "upf" ]; then
    echo "EXPOSE 8805/udp" >> "$svc/Dockerfile"
    echo "EXPOSE 2152/udp" >> "$svc/Dockerfile"
  else
    echo "EXPOSE $port" >> "$svc/Dockerfile"
  fi

  # Add CMD line
  echo "CMD [\"./$svc\"]" >> "$svc/Dockerfile"
done

echo "✅ All Dockerfiles updated to template."
echo "⚠️  Please review the changes before committing." 