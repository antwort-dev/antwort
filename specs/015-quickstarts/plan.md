# Implementation Plan: Quickstart Series

**Branch**: `015-quickstarts` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)

## Summary

Progressive quickstart series from minimal to RAG. Each quickstart is a self-contained directory with Kustomize manifests, ConfigMap, and README. Shared LLM backend and 01-minimal already done.

## Technical Context

**Format**: YAML manifests (Kustomize), Markdown READMEs
**No Go code**: Quickstarts are deployment artifacts, not application code
**Blockers**: 03 needs JWT auth (Spec 007 P2), 05 needs token exchange (Spec 10b), 06 needs RAG MCP server

## Project Structure

```
quickstarts/
├── shared/llm-backend/   (Done)
├── 01-minimal/           (Done)
├── 02-production/
│   ├── base/
│   │   ├── kustomization.yaml
│   │   ├── antwort-configmap.yaml
│   │   ├── antwort-deployment.yaml (patches 01)
│   │   └── antwort-service.yaml
│   ├── postgres/
│   ├── monitoring/       (Prometheus + Grafana)
│   ├── openshift/
│   └── README.md
├── 03-multi-user/        (blocked: JWT auth)
├── 04-mcp-tools/
│   ├── base/
│   ├── mcp-server/
│   ├── openshift/
│   └── README.md
├── 05-mcp-secured/       (blocked: token exchange)
└── 06-rag/               (blocked: RAG MCP server)
```
