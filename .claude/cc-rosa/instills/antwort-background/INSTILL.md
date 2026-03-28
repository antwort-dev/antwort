---
name: Antwort (Background)
id: antwort-background
description: Gateway + worker deployment for async background responses
requires: [model]
optional: []
params:
  - key: namespace
    description: Kubernetes namespace for Antwort deployment
    default: antwort
  - key: backend_url
    description: vLLM backend URL (auto-detected if empty)
    default: ""
    source: default
  - key: postgres_url
    description: PostgreSQL connection string (required for background mode)
    default: ""
    source: prompt
  - key: workers
    description: Number of background worker replicas
    default: "1"
---

# Antwort (Background)

Deploys Antwort in distributed mode from quickstart 09-background.
Runs a gateway (accepts requests) and worker(s) (processes background jobs) sharing a PostgreSQL queue.

## What Gets Installed

- Antwort gateway Deployment (mode=gateway)
- Antwort worker Deployment (mode=worker, configurable replicas)
- PostgreSQL StatefulSet (if no external postgres_url provided)
- Services and OpenShift Route for the gateway

## Prerequisites

- A model must be deployed via the `model` instill
- PostgreSQL 14+ (co-located or external)

## Notes

- Background mode requires PostgreSQL for the job queue
- Gateway handles HTTP requests and queues background work
- Workers poll the queue, process inference, and write results back
- Workers use heartbeat and stale detection for fault tolerance
