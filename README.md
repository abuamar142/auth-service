# Auth Service

Central authentication microservice for all abuamar.online projects. Handles user registration, login, JWT token management, and API key management.

## Stack

- Go 1.23+ with Chi router
- PostgreSQL 17
- JWT (HS256) access + refresh tokens
- bcrypt password hashing
- golang-migrate for DB migrations
- Docker multi-stage build

## Quick Start (Local)

```bash
cp .env.example .env
# Edit .env with your values

# Run with Docker
docker compose up --build

# Or run directly (requires PostgreSQL)
go run ./cmd/auth
```

## API Endpoints

### Public

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check (unversioned) |
| `POST` | `/api/v1/auth/register` | Create account (email OR username + password) |
| `POST` | `/api/v1/auth/login` | Login (email/username + password) → JWT + refresh token |
| `POST` | `/api/v1/auth/refresh` | Rotate refresh token → new JWT + refresh token |

### Protected (Bearer JWT)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/logout` | Revoke refresh token |
| `GET` | `/api/v1/auth/me` | Get current user |
| `GET` | `/api/v1/api-keys` | List API keys |
| `POST` | `/api/v1/api-keys` | Create API key |
| `DELETE` | `/api/v1/api-keys/{id}` | Delete API key |

### Response Format

All responses use a consistent envelope:

```json
{
  "success": true,
  "message": "login successful",
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "abc..."
  }
}
```

Errors:

```json
{
  "success": false,
  "message": "email or username is required",
  "error": {
    "code": "VALIDATION_ERROR",
    "details": ""
  }
}
```

### Request/Response Examples

**Register:**
```json
POST /api/v1/auth/register
{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "securepass123",
  "display_name": "John Doe"
}
→ 201: {"success": true, "message": "user registered", "data": {"id": "...", ...}}
```

**Login:**
```json
POST /api/v1/auth/login
{"identifier": "user@example.com", "password": "securepass123"}
→ 200: {"success": true, "message": "login successful", "data": {"access_token": "eyJ...", "refresh_token": "abc..."}}
```

**Refresh:**
```json
POST /api/v1/auth/refresh
{"refresh_token": "abc..."}
→ 200: {"success": true, "message": "token refreshed", "data": {"access_token": "eyJ...", "refresh_token": "def..."}}
```

## Tokens

- **Access token:** JWT HS256, 15 min expiry, passed as `Authorization: Bearer <token>`
- **Refresh token:** Random 64-byte hex, 7 days expiry, stored as SHA-256 hash in DB
- **API keys:** Random 32-byte hex prefixed with `ak_`, passed as `X-API-Key` header

## Consuming Apps

Other services validate JWT tokens locally using the shared `JWT_SECRET`:

```go
token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
    return []byte(jwtSecret), nil
})
claims := token.Claims.(jwt.MapClaims)
userID := claims["sub"]
```

No network call to auth service needed — just share `JWT_SECRET` across services.

## Deploy (VPS)

```bash
# Clone to /opt/auth-service/
git clone <repo> /opt/auth-service
cd /opt/auth-service

# Create .env (chmod 600)
cp .env.example .env
# Generate secrets: openssl rand -hex 64 (for JWT_SECRET)
# Generate password: openssl rand -base64 32 (for POSTGRES_PASSWORD)

# Production
docker compose --env-file .env -f docker-compose.yml -f docker-compose.prod.yml up -d --remove-orphans

# Development
docker compose --env-file .env -f docker-compose.yml -f docker-compose.dev.yml up -d --remove-orphans
```

## Structure

```
cmd/auth/main.go             — entry point
internal/
  config/config.go           — env-based config
  db/db.go                   — PostgreSQL connection pool
  migrations/embed.go        — embedded SQL files
  models/{user,token,apikey}.go — data models
  services/auth.go           — business logic (register, login, JWT, bcrypt)
  middleware/{auth,apikey}.go — JWT + API key validation
  handlers/{auth,apikey,health}.go — HTTP request/response
cmd/auth/migrations/         — SQL migration files
```
