---
name: Antwort (Minimal)
id: antwort-minimal
description: Stateless OpenResponses gateway with in-memory storage against vLLM
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
---

# Antwort (Minimal)

Deploys Antwort as a minimal stateless gateway using the quickstart 01-minimal kustomize overlay with OpenShift route exposure.
Uses in-memory storage and connects to the deployed vLLM InferenceService.

## What Gets Installed

- Antwort Deployment (single replica) in the configured namespace
- Service exposing port 8080
- OpenShift Route for external access
- ConfigMap with backend URL pointing to the vLLM model endpoint

## Prerequisites

- A model must be deployed via the `model` instill (vLLM InferenceService)
- cluster-admin or namespace-admin privileges
- `oc` CLI authenticated to the cluster

## Notes

- This is the simplest deployment pattern, suitable for validation testing
- No persistent storage: responses are not saved across restarts
- For production use, see `antwort-production` instill
