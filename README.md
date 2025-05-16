# OpenMVCore

A modular, Go-based 4G/5G MVNE core platform.

## Overview

OpenMVCore is a clean, hackable, and scalable alternative to Open5GS and Free5GC, built specifically for:
- Mobile Virtual Network Enablers (MVNEs)
- Private 5G deployments
- Research and development labs
- Startups building mobile services

## Services

- `smf/`: Session Management Function (GTPv2-C)
  - Handles PDU session establishment
  - Manages UE IP allocation
  - Controls UPF selection
- `amf/`: Access and Mobility Function
  - UE registration and authentication
  - Mobility management
  - NAS signaling
- `upf/`: User Plane Function
  - GTP-U tunneling
  - Packet forwarding
  - QoS enforcement
- `ocs/`: Online Charging System
  - Real-time charging
  - Quota management
  - Balance tracking
- `bss/`: Business Support System
  - REST API for subscriber management
  - Plan and subscription handling
  - SIM provisioning

## Quick Start

### Prerequisites
- Go 1.21 or later
- Docker and Docker Compose
- Make (optional)

### Run SMF Service

1. Start all services with Docker Compose:
```bash
docker-compose up -d
```

2. Or run SMF directly:
```bash
cd smf
go run main.go
```

### Development

1. Build all services:
```bash
make build
```

2. Run tests:
```bash
make test
```

3. Format code:
```bash
make fmt
```

## Architecture

The platform is built as a collection of microservices that implement different components of the 4G/5G core network. Each service is:
- Self-contained with embedded configuration
- Independently deployable
- Stateless (using Redis for session storage)
- Containerized with Docker

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
