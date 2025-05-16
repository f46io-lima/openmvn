# OCS (Online Charging System)

A lightweight Online Charging System for the OpenMVCore platform that provides real-time quota authorization and balance management for data sessions.

## Features

- Real-time quota authorization
- Balance management
- Subscriber balance tracking
- Health check endpoint
- Structured JSON logging
- Graceful shutdown
- Request logging and panic recovery

## API Endpoints

### Quota Authorization

`POST /quota`

Request quota authorization for a data session.

Request:
```json
{
  "imsi": "001010123456789",
  "mb": 20
}
```

Response (approved):
```json
{
  "approved": true,
  "balance": 80
}
```

Response (rejected):
```json
{
  "approved": false,
  "reason": "Insufficient balance",
  "balance": 10
}
```

### Get Balance

`GET /balance/{imsi}`

Retrieve current balance and usage information for a subscriber.

Response:
```json
{
  "imsi": "001010123456789",
  "balance": 80,
  "updated": "2024-03-21T10:00:00Z",
  "last_used": "2024-03-21T10:15:00Z"
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
docker build -t openmvcore-ocs .
docker run -p 8084:8084 openmvcore-ocs
```

### Docker Compose

The service is integrated into the project's Docker Compose setup:

```bash
docker-compose up ocs
```

## Integration

The OCS service is designed to be called by:
- SMF for session authorization
- BSS for balance updates
- External systems for quota management

## Usage Example

```bash
# Request quota authorization
curl -X POST http://localhost:8084/quota \
  -H "Content-Type: application/json" \
  -d '{"imsi":"001010123456789","mb":20}'

# Check balance
curl http://localhost:8084/balance/001010123456789
```

## Future Enhancements

1. PostgreSQL integration for persistent storage
2. Redis caching for high-performance lookups
3. Rate limiting
4. Metrics collection (Prometheus)
5. OpenAPI documentation
6. Support for multiple quota types
7. Usage statistics and reporting
8. Quota reservation system
9. Batch quota updates
10. Quota usage alerts 