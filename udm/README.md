# UDM (Unified Data Management)

A lightweight Unified Data Management service for the OpenMVCore platform that handles IMSI authentication and user data management.

## Features

- REST API for IMSI/token authentication
- In-memory user database (PostgreSQL-ready)
- Health check endpoint
- Graceful shutdown
- Request logging
- Panic recovery

## API Endpoints

### Authentication

`POST /auth`

Authenticates an IMSI/token pair.

Request:
```json
{
  "imsi": "001010123456789",
  "token": "secret123"
}
```

Response (success):
```json
{
  "authorized": true
}
```

Response (failure):
```json
{
  "authorized": false,
  "reason": "Invalid IMSI or token"
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
docker build -t openmvcore-udm .
docker run -p 8082:8082 openmvcore-udm
```

### Docker Compose

The service is integrated into the project's Docker Compose setup:

```bash
docker-compose up udm
```

## Integration

The UDM service is designed to be called by:
- AMF for UE authentication
- SMF for session authorization
- Other services requiring IMSI validation

## Future Enhancements

1. PostgreSQL integration
2. Redis caching
3. JWT token support
4. Rate limiting
5. Metrics collection
6. OpenAPI documentation 