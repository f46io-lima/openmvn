# UPF (User Plane Function)

A 5G User Plane Function implementation using `go-upf` that handles PFCP sessions and GTP-U traffic forwarding.

## Features

- PFCP (Packet Forwarding Control Protocol) support
  - Listens on port 8805
  - Handles session establishment/modification/deletion
  - Supports PDR (Packet Detection Rules) and FAR (Forwarding Action Rules)
- GTP-U (GPRS Tunneling Protocol - User Plane) support
  - Listens on port 2152
  - Handles user data packet forwarding
  - Uses TUN interface for packet routing
- Session management
  - Tracks active PFCP sessions
  - Monitors session statistics
  - Supports graceful shutdown

## Prerequisites

- Linux kernel with TUN/TAP support
- Docker with NET_ADMIN capability
- Network tools (iproute2, tcpdump)

## Building and Running

### Local Build

```bash
# Build
go build -o upf main.go

# Run (requires root for TUN device)
sudo ./upf
```

### Docker

```bash
# Build and run with Docker
docker build -t openmvcore-upf .
docker run --cap-add=NET_ADMIN -p 8805:8805/udp -p 2152:2152/udp openmvcore-upf
```

### Docker Compose

The service is integrated into the project's Docker Compose setup:

```bash
docker-compose up upf
```

## Configuration

The UPF is configured through the `UPFConfig` struct in `main.go`:

```go
cfg := &context.UPFConfig{
    PFCP: context.PFCPConfig{
        Addr: "0.0.0.0:8805",  // PFCP interface
    },
    GTP: context.GTPConfig{
        Addr: "0.0.0.0:2152",  // GTP-U interface
    },
    EnableGTP: true,
    EnablePFCP: true,
    EnableUPlane: true,
    ReportNotify: true,
    LogLevel: "info",
}
```

## Integration with SMF

The UPF expects PFCP messages from the SMF on port 8805. The SMF should:

1. Establish PFCP association
2. Create/modify/delete sessions
3. Install PDRs and FARs
4. Monitor session status

## Monitoring

The UPF provides:
- Regular status updates (every 30 seconds)
- Session count monitoring
- Detailed logging via logrus
- TUN interface statistics

## Next Steps

1. Add metrics collection
2. Implement QoS handling
3. Add support for multiple TUN interfaces
4. Enhance session monitoring
5. Add configuration file support
6. Implement UPF selection logic 