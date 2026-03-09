---
name: Antwort (Production)
id: antwort-production
description: Stateful OpenResponses gateway with PostgreSQL storage
requires: [model]
optional: []
params:
  - key: namespace
    description: Kubernetes namespace for Antwort deployment
    default: antwort
  - key: backend_url
    description: vLLM backend URL (auto-detected from model InferenceService if empty)
    default: ""
    source: default
  - key: replicas
    description: Number of Antwort replicas
    default: "2"
  - key: postgres_url
    description: PostgreSQL connection string
    default: ""
    source: prompt
---

# Antwort (Production)

Deploys Antwort with PostgreSQL-backed storage using the quickstart 02-production configuration.
Supports response persistence, conversation chaining, and multi-replica deployments.

## What Gets Installed

- Antwort Deployment (configurable replicas) in the configured namespace
- PostgreSQL StatefulSet (if no external postgres_url provided)
- Service and OpenShift Route
- ConfigMap with backend URL and storage configuration

## Prerequisites

- A model must be deployed via the `model` instill
- For external PostgreSQL: a running PostgreSQL 14+ instance with connection string
- cluster-admin or namespace-admin privileges

## Notes

- If `postgres_url` is empty, a single-replica PostgreSQL is deployed alongside Antwort
- For high availability, provide an external PostgreSQL (e.g., CrunchyData operator)
- Responses, conversations, and files persist across Antwort restarts
