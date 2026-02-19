# Quickstart: Authentication & Authorization

**Feature**: 007-auth
**Date**: 2026-02-19

## No Auth (Development Default)

```bash
# No auth config = all requests accepted (NoOp)
ANTWORT_BACKEND_URL=http://localhost:9090 go run ./cmd/server
```

## API Key Authentication

```bash
# Configure API keys via environment
ANTWORT_AUTH_TYPE=apikey \
ANTWORT_API_KEYS='[{"key":"sk-test-key-1","subject":"alice","tenant_id":"org-1","service_tier":"standard"}]' \
go run ./cmd/server

# Request with valid key
curl -H "Authorization: Bearer sk-test-key-1" \
  -X POST http://localhost:8080/v1/responses -d '...'

# Request without key -> 401
curl -X POST http://localhost:8080/v1/responses -d '...'
# {"error":{"type":"invalid_request","message":"authentication required"}}
```

## Auth Chain (API Key + JWT)

```bash
ANTWORT_AUTH_TYPE=chain \
ANTWORT_AUTH_CHAIN='apikey,jwt' \
ANTWORT_API_KEYS='[{"key":"sk-admin","subject":"admin"}]' \
ANTWORT_JWT_ISSUER=https://auth.example.com \
ANTWORT_JWT_AUDIENCE=antwort \
ANTWORT_JWT_JWKS_URL=https://auth.example.com/.well-known/jwks.json \
go run ./cmd/server

# API key users send Bearer sk-...
# JWT users send Bearer eyJ...
# Both work through the same endpoint
```

## Rate Limiting

```bash
ANTWORT_RATE_LIMITS='{"standard":{"requests_per_minute":60},"premium":{"requests_per_minute":600}}' \
go run ./cmd/server

# 61st request within a minute -> 429
```

## Health Endpoint Bypass

```bash
# Health endpoints work without auth
curl http://localhost:8080/healthz  # 200 OK (no auth needed)
curl http://localhost:8080/readyz   # 200 OK (no auth needed)
```
