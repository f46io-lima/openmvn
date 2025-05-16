#!/bin/bash
set -e

echo "🔧 Starting module patching for all services..."

for svc in amf smf ocs udm upf bss; do
  echo "🔧 Patching $svc..."
  cd $svc
  go get github.com/nats-io/nats.go@v1.33.1
  go get github.com/nats-io/nkeys@v0.4.7
  go get golang.org/x/crypto/curve25519
  go get golang.org/x/crypto/ed25519
  go get golang.org/x/crypto/nacl/box
  go get github.com/cespare/xxhash/v2@v2.2.0
  go mod tidy
  go mod download
  cd ..
done

echo "✅ All services patched and go.sum updated." 