# AMF (Access & Mobility Function)

A simplified 5G AMF implementation that handles UE registration and mobility management through SCTP/NGAP.

## Features

- SCTP server on port 38412 (standard NGAP port)
- ASN.1-based message encoding/decoding
- Simulated UDM authentication
- In-memory UE context management
- Support for multiple message types:
  - NG Setup Request/Response
  - Initial UE Message
- Concurrent connection handling

## Protocol Support

### NGAP (Next Generation Application Protocol)
- Listens on SCTP port 38412
- Implements simplified ASN.1 message format:
  ```go
  type NGSetupRequest struct {
      MessageType int
      UEID       uint64
      NASData    []byte
      Timestamp  time.Time
  }
  ```
- Handles message types:
  1. NG Setup Request (1)
  2. NG Setup Response (2)
  3. Initial UE Message (3)

### Authentication
- Simulated UDM authentication
- NAS data validation (currently a simple byte check)
- Authentication status tracking in UE context

## UE Context

The service maintains UE context information including:
- UE ID (uint64)
- gNodeB address
- Authentication status
- Registration status
- Last seen timestamp
- Creation timestamp
- Additional fields for future use (SUPI, AMF ID, etc.)

## Building and Running

```bash
# Build
go build -o amf

# Run
./amf
```

## Testing with gNB Simulator

The project includes a `gnb-sim` tool for testing:

```bash
# Build gNB simulator
cd gnb-sim
go build -o gnb-sim

# Run simulator
./gnb-sim
```

The simulator will:
1. Connect to AMF on localhost:38412
2. Generate a random UE ID
3. Send a simulated NG Setup Request
4. Wait for and log the response

## Next Steps

1. Implement full NGAP ASN.1 message set
2. Add proper NAS message handling
3. Integrate with real UDM for authentication
4. Add persistent storage
5. Implement proper session management
6. Add metrics and monitoring
7. Add support for more NGAP procedures

## Docker

The service is containerized and can be run using Docker Compose:

```bash
docker-compose up amf
```

## Development

To add new message types:
1. Define the message struct
2. Add message type constant
3. Implement handler function
4. Add case to message type switch in `handleNG` 