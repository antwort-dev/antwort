# Brainstorm 15: API Conformance & Integration Testing

**Dependencies**: Spec 006 (Conformance), all API-surface specs
**Package**: N/A (test infrastructure)

## Purpose

Establish a comprehensive API conformance testing pipeline that validates antwort's API surface against both the OpenResponses spec and our own OpenAPI schema. Two complementary tools: oasdiff for schema structure, Zod compliance suite for runtime behavior.

## Architecture

```
┌─────────────────────────────────────────────────┐
│ API Conformance Pipeline                         │
│                                                  │
│  1. OpenAPI Schema Validation (oasdiff)           │
│     ├── antwort.openapi.yaml (our spec)           │
│     └── openresponses.openapi.json (upstream)     │
│     Result: breaking changes, missing endpoints   │
│                                                  │
│  2. Runtime Conformance (Zod suite)               │
│     ├── antwort server + mock LLM backend         │
│     └── official compliance tests                 │
│     Result: response schema compliance            │
│                                                  │
│  3. Integration Tests (Go, container-based)       │
│     ├── Full API surface testing                  │
│     └── Side-API validation (vector stores, etc.) │
│     Result: endpoint coverage, error handling     │
└─────────────────────────────────────────────────┘
```

## Our OpenAPI Spec

We maintain our own `openapi.yaml` that covers:

1. **OpenResponses endpoints** (POST/GET/DELETE /v1/responses) - must align with upstream
2. **Side-APIs** (Vector Store, health, metrics) - our extensions
3. **Streaming events** - SSE event schema documentation

Structure:
```
api/
├── openapi.yaml              # Full antwort API spec
├── openresponses-ref.json    # Upstream OpenResponses spec (downloaded)
└── validate.sh               # Run oasdiff + Zod + integration tests
```

The OpenResponses portion of our spec is validated against the upstream via oasdiff. If we diverge, oasdiff reports it. This gives us:
- A single source of truth for our full API surface
- Automatic detection of upstream spec changes we need to incorporate
- Documentation for side-APIs that aren't in OpenResponses

## Container-Based Testing

All tests run in containers for GitHub Actions compatibility:

```yaml
# GitHub Actions workflow
- name: API Conformance
  run: |
    podman build -t antwort-test -f test/Containerfile .
    podman run --rm antwort-test
```

The test container:
1. Builds antwort + mock backend from source
2. Starts both services
3. Runs oasdiff against the OpenAPI spec
4. Runs Zod compliance suite
5. Runs Go integration tests against all endpoints
6. Reports combined results

## oasdiff Usage

```bash
# Download upstream spec
curl -o openresponses-ref.json \
  https://raw.githubusercontent.com/openresponses/openresponses/main/public/openapi/openapi.json

# Check for breaking changes (our spec vs upstream)
oasdiff breaking openresponses-ref.json openapi.yaml

# Generate diff report
oasdiff diff openresponses-ref.json openapi.yaml -f markdown
```

## Integration Test Scenarios

| Category | Test | Method |
|----------|------|--------|
| Responses | Create (non-streaming) | POST /v1/responses |
| Responses | Create (streaming) | POST /v1/responses (SSE) |
| Responses | Get by ID | GET /v1/responses/{id} |
| Responses | Delete | DELETE /v1/responses/{id} |
| Responses | Not found | GET /v1/responses/invalid |
| Auth | Unauthenticated | POST without token → 401 |
| Auth | Invalid token | POST with bad token → 401 |
| Health | Liveness | GET /healthz → 200 |
| Metrics | Prometheus | GET /metrics → prometheus format |
| Vector Stores | Create | POST /v1/vector_stores |
| Vector Stores | List | GET /v1/vector_stores |
| Vector Stores | Delete | DELETE /v1/vector_stores/{id} |
| Streaming | Event sequence | response.created → content → completed |
| Error | Invalid request | POST with bad JSON → 400 |
| Error | Missing model | POST without model → 400 |

## Decisions

- Own OpenAPI spec from day one (covers OpenResponses + side-APIs)
- Container-based for CI (GitHub Actions, no host dependencies)
- Both oasdiff (schema) and Zod (runtime) validation
- Integration tests in Go (compiled into test container)
- oasdiff checks OpenResponses alignment specifically

## Deliverables

- [ ] `api/openapi.yaml` - Full antwort API spec
- [ ] oasdiff validation against upstream OpenResponses spec
- [ ] Container-based test runner (Containerfile)
- [ ] Go integration tests for all endpoints
- [ ] GitHub Actions workflow
- [ ] Makefile target: `make api-test`
