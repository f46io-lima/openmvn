#!/bin/bash
set -e

SERVICES=(amf smf ocs bss udm upf)

for svc in "${SERVICES[@]}"; do
  echo "🔧 Patching $svc..."
  cd $svc
  go get github.com/nats-io/nats.go@v1.33.1
  go get github.com/nats-io/nkeys@v0.4.7
  go get github.com/cespare/xxhash/v2@v2.2.0
  go get github.com/klauspost/compress/flate
  go get golang.org/x/crypto/curve25519
  go get golang.org/x/crypto/ed25519
  go get golang.org/x/crypto/nacl/box
  go mod tidy
  go mod download
  cd ..
done

echo "✅ All services patched. Commit your go.mod and go.sum files." 