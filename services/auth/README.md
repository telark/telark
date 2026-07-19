# Auth Service

A Go-based WebAuthn/Passkeys authentication backend service for passwordless authentication in Kubernetes clusters.

## Overview

This service provides secure, passwordless authentication using WebAuthn and passkeys (FIDO2). It integrates with the exporter-service for CRD operations and follows the telark service architecture patterns.

## Features

- **Passwordless Authentication**: WebAuthn-based authentication using passkeys
- **Session Management**: Secure session token generation and validation
- **Passkey Management**: CRUD operations for user passkeys
- **Kubernetes-Native**: Designed to run in Kubernetes with CRD-based storage
- **Graceful Shutdown**: Proper signal handling and graceful server shutdown
- **Health Checks**: Built-in health, readiness, and liveness endpoints

## Architecture

### Service Dependencies

- **exporter-service**: Provides REST API for CRD operations (Users, UserSession, AuthChallenge, UserPasskey)
- **data package** (`github.com/telark/data`): Data structures and CRD types
- **rest package** (`github.com/telark/rest`): REST clients, endpoints, and router
- **kcore package** (`github.com/telark/kcore`): Kubernetes client utilities

## Configuration

### Environment Variables

The service requires the following environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RP_ID` | Yes | - | WebAuthn Relying Party ID (domain) |
| `RP_NAME` | Yes | - | WebAuthn Relying Party display name |
| `RP_ORIGIN` | Yes | - | WebAuthn Relying Party origin URL |
| `CHALLENGE_TIMEOUT` | No | 60 | Challenge expiration time in seconds |
| `SESSION_EXPIRY` | No | 24 | Session expiration time in hours |
| `PORT` | No | 8080 | HTTP server port |

### Helm Configuration

The service is configured in the release-manager Helm values at `/Users/houssem/Desktop/Github/release-manager/helm/values.yaml`:

```yaml
authService:
  enabled: false  # Set to true to enable
  name: "auth-service"
  tag: "auth-"
  version: 0.0.1
  type: "auth"
  replicas: 1
  port: 8080
  serviceType: "ClusterIP"
  env:
    RP_ID: "localhost"
    RP_NAME: "Dashboard App"
    RP_ORIGIN: "http://localhost:3000"
    CHALLENGE_TIMEOUT: "60"
    SESSION_EXPIRY: "24"
  portForward:
    enabled: false
    localPort: 8006
```

## API Endpoints

### Authentication Endpoints

#### POST `/api/v1/auth/login/start`
Initiates the WebAuthn authentication flow.

**Request Body:**
```json
{
  "username": "user@example.com"
}
```

**Response:**
```json
{
  "options": {
    "challenge": "...",
    "allowCredentials": [...]
  }
}
```

#### POST `/api/v1/auth/login/finish`
Completes the WebAuthn authentication flow.

**Request Body:**
```json
{
  "username": "user@example.com",
  "response": {
    // WebAuthn credential assertion response
  }
}
```

**Response:**
```json
{
  "sessionToken": "uuid-token",
  "user": {
    "id": "user-id",
    "username": "user@example.com",
    ...
  }
}
```

#### POST `/api/v1/auth/logout`
Invalidates the user's session.

**Headers:**
- `X-Session-Token`: Session token to invalidate

**Response:**
```json
{
  "success": true,
  "message": "Logout completed successfully"
}
```

### Passkey Management Endpoints

All passkey endpoints require authentication via `X-Session-Token` header.

#### GET `/api/v1/auth/passkeys/proxy/get`
Returns all passkeys for the authenticated user.

**Headers:**
- `X-Session-Token`: User's session token

**Response:**
```json
[
  {
    "credentialId": "...",
    "deviceName": "MacBook Pro",
    "deviceType": "platform",
    "creationTimestamp": "2025-01-01T00:00:00Z",
    "lastUsedTimestamp": "2025-01-02T00:00:00Z"
  }
]
```

#### POST `/api/v1/auth/passkeys/proxy/create`
Creates a new passkey for the authenticated user.

**Headers:**
- `X-Session-Token`: User's session token

**Request Body:**
```json
{
  "response": {
    // WebAuthn credential creation response
  },
  "deviceName": "MacBook Pro",
  "deviceType": "platform"
}
```

#### GET `/api/v1/auth/passkeys/proxy/single/get`
Returns a specific passkey.

**Headers:**
- `X-Session-Token`: User's session token
- `X-Credential-ID`: Credential ID to retrieve

#### PATCH `/api/v1/auth/passkeys/proxy/patch`
Updates a specific passkey.

**Headers:**
- `X-Session-Token`: User's session token
- `X-Credential-ID`: Credential ID to update

**Request Body:**
```json
{
  "deviceName": "New Device Name"
}
```

#### DELETE `/api/v1/auth/passkeys/proxy/delete`
Deletes a specific passkey.

**Headers:**
- `X-Session-Token`: User's session token
- `X-Credential-ID`: Credential ID to delete

**Request Body:**
```json
{
  "forceLastDelete": false
}
```

### Health Check Endpoints

- `GET /health` - Service health check
- `GET /ready` - Service readiness check
- `GET /live` - Service liveness check

## Development

### Prerequisites

- Go 1.25 or later
- Access to Kubernetes cluster
- exporter-service running and accessible

### Building

```bash
# Install dependencies
go mod tidy

# Build the service
go build -o auth-service .
```

### Running Locally

```bash
# Set required environment variables
export RP_ID="localhost"
export RP_NAME="My App"
export RP_ORIGIN="http://localhost:3000"
export CHALLENGE_TIMEOUT="60"
export SESSION_EXPIRY="24"
export PORT="8080"

# Run the service
./auth-service
```

### Running in Kubernetes

1. Build and push Docker image:
```bash
docker build -t telark/auth:0.3.2 .
docker push telark/auth:0.3.2
```

2. Enable service in Helm values:
```yaml
authService:
  enabled: true  # Change from false to true
```

3. Deploy via Helm:
```bash
cd /Users/houssem/Desktop/Github/release-manager
helm upgrade --install telark-release ./helm
```

## Security Considerations

### Authentication Flow

1. **Challenge Storage**: Challenges are stored with expiration and deleted after use
2. **Session Management**: Sessions have configurable expiration times
3. **Credential Verification**: WebAuthn credentials are verified using cryptographic signatures
4. **No Password Storage**: Service uses passwordless authentication only

### Best Practices

- Set appropriate `CHALLENGE_TIMEOUT` (recommended: 60 seconds)
- Set appropriate `SESSION_EXPIRY` (recommended: 24 hours)
- Use HTTPS in production (configure `RP_ORIGIN` with https://)
- Configure `RP_ID` to match your domain
- Implement rate limiting at the ingress level
- Monitor failed authentication attempts

## Error Handling

The service uses consistent error responses:

```json
{
  "error": true,
  "message": "Error description"
}
```

Common error status codes:
- `400` - Bad Request (invalid input)
- `401` - Unauthorized (invalid/missing session)
- `404` - Not Found (user/passkey not found)
- `500` - Internal Server Error

## Logging

The service uses structured JSON logging with the following levels:
- `INFO`: Normal operations
- `WARN`: Warnings that don't affect operations
- `ERROR`: Errors that need attention

All logs include:
- Service name
- Timestamp
- Log level
- Message

## Testing

### Unit Tests

```bash
go test ./... -v
```

### Integration Tests

Integration tests require a running exporter-service and Kubernetes cluster.

```bash
go test ./... -tags=integration -v
```

## Troubleshooting

### Common Issues

**Issue: WebAuthn initialization fails**
- Verify `RP_ID`, `RP_NAME`, and `RP_ORIGIN` are set correctly
- Ensure `RP_ORIGIN` includes the protocol (http:// or https://)

**Issue: Session validation fails**
- Check that the session hasn't expired
- Verify the session token is being sent in the `X-Session-Token` header
- Ensure exporter-service is accessible

**Issue: Passkey creation fails**
- Verify the WebAuthn credential response is valid
- Check that the user has permission to create passkeys
- Review exporter-service logs for CRD operation errors

## Contributing

Follow the telark coding standards:
- Follow SOLID principles
- Keep functions under 15 cognitive complexity
- Write comprehensive tests
- Document all public APIs
- Use the existing error constants

## License

See LICENSE.md file for details.

## Support

For issues and questions, please refer to the project documentation or contact the development team.

