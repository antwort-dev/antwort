# Quickstart: Background Workers (Gateway + Worker Deployment)

**Feature**: 044-async-responses

## Overview

This quickstart demonstrates deploying Antwort with separate gateway and worker components for asynchronous background processing. The gateway accepts HTTP requests and queues background work. Workers poll for queued requests and process them independently. Both share a PostgreSQL database.

## Prerequisites

- Kubernetes cluster (or local kind/minikube)
- PostgreSQL database accessible from the cluster
- Antwort container image

## Architecture

```
                    ┌─────────────┐
   Client ─────────>│   Gateway   │──── sync responses ────> Client
                    │ (HTTP only) │
                    └──────┬──────┘
                           │ background: true
                           │ (queues to PostgreSQL)
                    ┌──────v──────┐
                    │ PostgreSQL  │
                    └──────┬──────┘
                           │ polls for queued
                    ┌──────v──────┐
                    │   Worker    │──── processes ────> updates PostgreSQL
                    │ (no HTTP)   │
                    └─────────────┘
```

## Deployment Components

### Gateway Deployment

- Runs with `--mode=gateway`
- Exposes HTTP port for client requests
- Handles synchronous requests normally
- Queues background requests to PostgreSQL
- Scales based on HTTP traffic

### Worker Deployment

- Runs with `--mode=worker`
- No HTTP port exposed
- Polls PostgreSQL for queued background requests
- Processes requests through the full engine pipeline
- Scales based on background workload

### Integrated Mode (Development)

- Runs with `--mode=integrated` (default)
- Single process handles both HTTP and background processing
- No PostgreSQL required (in-memory storage)
- Suitable for local development and testing

## Configuration

### Gateway config.yaml

```yaml
engine:
  mode: gateway
  backend_url: http://vllm:8000
  default_model: meta-llama/Llama-3.1-8B-Instruct

storage:
  type: postgres
  postgres:
    dsn: postgres://antwort:password@postgres:5432/antwort
    migrate_on_start: true

server:
  port: 8080
```

### Worker config.yaml

```yaml
engine:
  mode: worker
  backend_url: http://vllm:8000
  default_model: meta-llama/Llama-3.1-8B-Instruct
  background:
    poll_interval: 5s
    drain_timeout: 30s
    staleness_timeout: 10m
    heartbeat_interval: 30s
    ttl: 24h

storage:
  type: postgres
  postgres:
    dsn: postgres://antwort:password@postgres:5432/antwort
```

## Usage

### Submit a background request

```bash
curl -X POST http://gateway:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "input": [{"role": "user", "content": "Write a detailed analysis of..."}],
    "background": true
  }'
```

Response (immediate):
```json
{
  "id": "resp_abc123...",
  "status": "queued",
  "background": true,
  "output": []
}
```

### Poll for result

```bash
curl http://gateway:8080/v1/responses/resp_abc123...
```

### List background requests

```bash
curl "http://gateway:8080/v1/responses?background=true&status=queued"
```

### Cancel a background request

```bash
curl -X DELETE http://gateway:8080/v1/responses/resp_abc123...
```

## Scaling

- Scale gateway pods for HTTP throughput: `kubectl scale deployment antwort-gateway --replicas=3`
- Scale worker pods for background capacity: `kubectl scale deployment antwort-worker --replicas=8`
- Gateway and worker pods are independent; scaling one does not affect the other
