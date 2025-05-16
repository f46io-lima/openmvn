# BSS (Business Support System)

A lightweight Business Support System for the OpenMVCore platform that handles SIM registration, balance management, and subscriber information.

## Features

- SIM registration with IMSI and token
- Balance top-up management
- Subscriber information retrieval
- Health check endpoint
- Structured JSON logging
- Graceful shutdown
- Request logging and panic recovery

## API Endpoints

### Register New Subscriber

`POST /register`

Register a new SIM subscriber.

Request:
```json
{
  "imsi": "001010123456789",
  "token": "secret123",
  "balance": 100
}
```

Response (success):
```json
{
  "imsi": "001010123456789",
  "token": "secret123",
  "balance": 100,
  "created": "2024-03-21T10:00:00Z",
  "updated": "2024-03-21T10:00:00Z"
}
```

### Top Up Balance

`POST /topup`

Add balance to a subscriber's account.

Request:
```json
{
  "imsi": "001010123456789",
  "amount": 50
}
```

Response (success):
```json
{
  "imsi": "001010123456789",
  "balance": 150,
  "message": "Top-up successful"
}
```

### Get Subscriber Info

`GET /subscriber/{imsi}`

Retrieve subscriber information.

Response (success):
```json
{
  "imsi": "001010123456789",
  "token": "secret123",
  "balance": 150,
  "created": "2024-03-21T10:00:00Z",
  "updated": "2024-03-21T10:15:00Z"
}
```

### Health Check

`GET /health`

Returns `200 OK` if the service is healthy.

## Development

### Prerequisites

- Go 1.21 or later
- Docker (optional)

### Local Run

```bash
go mod download
go run main.go
```

### Docker Build

```bash
docker build -t openmvcore-bss .
docker run -p 8083:8083 openmvcore-bss
```

### Docker Compose

The service is integrated into the project's Docker Compose setup:

```bash
docker-compose up bss
```

## Integration

The BSS service is designed to be called by:
- UDM for subscriber validation
- OCS for balance checks
- External systems for SIM management

## Future Enhancements

1. PostgreSQL integration for persistent storage
2. Redis caching for high-performance lookups
3. JWT-based authentication for API access
4. Rate limiting
5. Metrics collection (Prometheus)
6. OpenAPI documentation
7. Support for multiple currencies
8. Transaction history
9. Plan management
10. Usage statistics 