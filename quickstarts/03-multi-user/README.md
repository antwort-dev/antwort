# Quickstart 03: Multi-User with JWT Authentication

Deploy antwort with Keycloak for JWT-based authentication and tenant isolation. Two users (alice and bob) are pre-configured in separate tenants, demonstrating that each user can only access their own data.

**Time to deploy**: 10 minutes (after LLM backend is running)

## Prerequisites

- [Shared LLM Backend](../shared/llm-backend/) deployed and running
- `kubectl` or `oc` CLI configured
- `curl` and `jq` for testing

## What's Deployed

| Component | Description |
|-----------|-------------|
| antwort | OpenResponses gateway with JWT auth (1 pod) |
| Keycloak | Identity provider with embedded H2 database, realm import (1 pod) |
| PostgreSQL | Database for antwort response storage (StatefulSet, 5Gi PVC) |
| ConfigMap | JWT auth config pointing to Keycloak OIDC |
| Secrets | PostgreSQL credentials, Keycloak admin credentials |
| Routes (OpenShift) | Edge TLS for antwort and Keycloak |

Keycloak runs in dev mode with its embedded H2 database, which simplifies the deployment (no external database dependency). Token exchange and admin fine-grained authorization features are pre-enabled for use in later quickstarts.

### Pre-configured Users

| User | Password | Tenant | Role |
|------|----------|--------|------|
| alice | alice123 | tenant-alice | standard |
| bob | bob123 | tenant-bob | premium |

## Deploy

```bash
# Create namespace
kubectl create namespace antwort

# Deploy everything (antwort + PostgreSQL + Keycloak)
kubectl apply -k quickstarts/03-multi-user/base/ -n antwort

# Wait for Keycloak to be ready (needs realm import, may take ~60s)
kubectl rollout status deployment/keycloak -n antwort --timeout=180s

# Wait for PostgreSQL and antwort to be ready
kubectl rollout status statefulset/postgres -n antwort --timeout=120s
kubectl rollout status deployment/antwort -n antwort --timeout=60s
```

### OpenShift / ROSA

For external access via Routes:

```bash
# Apply with OpenShift overlay (includes Routes for antwort and Keycloak)
kubectl apply -k quickstarts/03-multi-user/openshift/ -n antwort

# Get the route URLs
ANTWORT_ROUTE=$(kubectl get route antwort -n antwort -o jsonpath='{.spec.host}')
KEYCLOAK_ROUTE=$(kubectl get route keycloak -n antwort -o jsonpath='{.spec.host}')
echo "Antwort URL: https://$ANTWORT_ROUTE"
echo "Keycloak URL: https://$KEYCLOAK_ROUTE"
```

## Test

### Setup Port-Forwards

```bash
# Antwort API
kubectl port-forward -n antwort svc/antwort 8080:8080 &

# Keycloak (for token requests)
kubectl port-forward -n antwort svc/keycloak 8081:8080 &

export ANTWORT_URL=http://localhost:8080
export KEYCLOAK_URL=http://localhost:8081
```

### Get a JWT Token for Alice

```bash
ALICE_TOKEN=$(curl -s -X POST \
  "$KEYCLOAK_URL/realms/antwort/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=antwort-cli" \
  -d "username=alice" \
  -d "password=alice123" | jq -r '.access_token')

echo "Alice's token (first 50 chars): ${ALICE_TOKEN:0:50}..."
```

You can inspect the token claims at [jwt.io](https://jwt.io) to verify the `tenant_id` and `realm_roles` fields are present.

### Create a Response as Alice

```bash
ALICE_RESPONSE=$(curl -s -X POST "$ANTWORT_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "What is the capital of France? Answer in one sentence."}]
      }
    ]
  }')

ALICE_RESPONSE_ID=$(echo "$ALICE_RESPONSE" | jq -r '.id')
echo "Alice's response ID: $ALICE_RESPONSE_ID"
echo "$ALICE_RESPONSE" | jq '{id: .id, status: .status, answer: .output[0].content[0].text}'
```

### Verify Alice Can Retrieve Her Own Response

```bash
curl -s "$ANTWORT_URL/v1/responses/$ALICE_RESPONSE_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | \
  jq '{id: .id, status: .status, answer: .output[0].content[0].text}'
```

### Get a JWT Token for Bob

```bash
BOB_TOKEN=$(curl -s -X POST \
  "$KEYCLOAK_URL/realms/antwort/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=antwort-cli" \
  -d "username=bob" \
  -d "password=bob123" | jq -r '.access_token')

echo "Bob's token (first 50 chars): ${BOB_TOKEN:0:50}..."
```

### Verify Tenant Isolation: Bob Cannot Access Alice's Response

```bash
# Bob tries to retrieve Alice's response (should get 404)
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  "$ANTWORT_URL/v1/responses/$ALICE_RESPONSE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")

echo "HTTP status when Bob accesses Alice's response: $HTTP_STATUS"
# Expected: 404 (Not Found - tenant isolation prevents access)
```

### Verify Bob Can Create and Access His Own Responses

```bash
BOB_RESPONSE=$(curl -s -X POST "$ANTWORT_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "What is 2 + 2?"}]
      }
    ]
  }')

BOB_RESPONSE_ID=$(echo "$BOB_RESPONSE" | jq -r '.id')
echo "Bob's response ID: $BOB_RESPONSE_ID"

# Bob can access his own response
curl -s "$ANTWORT_URL/v1/responses/$BOB_RESPONSE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN" | \
  jq '{id: .id, status: .status}'
```

### Verify Unauthenticated Access is Denied

```bash
# Request without a token (should get 401)
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$ANTWORT_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "Hello"}]
      }
    ]
  }')

echo "HTTP status without token: $HTTP_STATUS"
# Expected: 401 (Unauthorized)
```

### Test Structured Output

Request a response with a JSON schema to get structured output:

```bash
curl -s -X POST "$ANTWORT_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "List 3 programming languages with their year of creation"}]
      }
    ],
    "text": {
      "format": {
        "type": "json_schema",
        "name": "languages",
        "schema": {
          "type": "object",
          "properties": {
            "languages": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": {"type": "string"},
                  "year": {"type": "integer"}
                },
                "required": ["name", "year"]
              }
            }
          },
          "required": ["languages"]
        }
      }
    }
  }' | jq '.output[] | select(.type == "message") | .content[0].text' -r | jq .
```

The `text.format` field constrains the model to produce valid JSON matching the schema. Expected output:

```json
{
  "languages": [
    {"name": "Python", "year": 1991},
    {"name": "JavaScript", "year": 1995},
    {"name": "Go", "year": 2009}
  ]
}
```

### Test Reasoning

Request a response with reasoning to see the model's thought process:

```bash
curl -s -X POST "$ANTWORT_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "What is 15% of 240?"}]
      }
    ],
    "reasoning": {"effort": "medium"}
  }' | jq '{
    output_types: [.output[].type],
    reasoning: [.output[] | select(.type == "reasoning") | .summary[0].text],
    answer: [.output[] | select(.type == "message") | .content[0].text]
  }'
```

Reasoning output depends on model support. If the model does not support reasoning, the response will complete normally without reasoning items.

## Keycloak Admin Console

Access the Keycloak admin console to manage users, clients, and realm settings:

```bash
# Via port-forward
open http://localhost:8081/admin

# Login: admin / admin-secret
```

On OpenShift, use the Keycloak Route URL instead.

## Configuration

The `config.yaml` in the ConfigMap enables JWT authentication via Keycloak:

```yaml
server:
  port: 8080

engine:
  provider: vllm
  backend_url: http://llm-predictor.llm-serving.svc.cluster.local:8080
  default_model: /mnt/models

storage:
  type: postgres
  postgres:
    dsn_file: /run/secrets/postgres/dsn
    migrate_on_start: true

auth:
  type: jwt
  jwt:
    issuer: http://keycloak:8080/realms/antwort
    audience: antwort-gateway
    jwks_url: http://keycloak:8080/realms/antwort/protocol/openid-connect/certs
    user_claim: sub
    tenant_claim: tenant_id
    scopes_claim: scope

observability:
  metrics:
    enabled: true
```

Key auth settings:

| Field | Description |
|-------|-------------|
| `issuer` | Expected JWT issuer (Keycloak realm URL) |
| `audience` | Expected `aud` claim (must match the Keycloak client) |
| `jwks_url` | Endpoint for fetching public keys to verify JWT signatures |
| `user_claim` | JWT claim that identifies the user (`sub`) |
| `tenant_claim` | JWT claim used for tenant isolation (`tenant_id`) |
| `scopes_claim` | JWT claim for authorization scopes |

## Next Steps

Ready for more? Continue to [Quickstart 04: MCP Tools](../04-mcp-tools/) to add MCP tool calling with an agentic loop.

## Cleanup

```bash
kubectl delete namespace antwort
```

This also removes the PostgreSQL PVC and all stored data. Keycloak's embedded H2 data is ephemeral and lost with the pod.
