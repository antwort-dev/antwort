# Feature Specification: Observability (Prometheus Metrics)

**Feature Branch**: `013-observability`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification adds Prometheus metrics to antwort for production monitoring. Metrics cover four domains: request handling, provider inference, tool execution, and rate limiting. Histogram buckets are tuned for LLM workloads (sub-second to two minutes) rather than typical web service latencies.

A `/metrics` endpoint exposes all metrics for Prometheus scraping. The OpenShift overlay already includes a ServiceMonitor (Spec 010) that scrapes this endpoint.

OpenTelemetry distributed tracing is deferred to a follow-up spec.

## Clarifications

### Session 2026-02-20

- Q: Prometheus or OpenTelemetry metrics? -> A: Prometheus for P1. It's the Kubernetes standard, already has ServiceMonitor in our OpenShift overlay, and the client library is lightweight. OpenTelemetry tracing is P2.
- Q: Where to record metrics? -> A: Transport middleware (request count/duration), engine (provider latency, tool executions), auth (rate limit rejections). Small instrumentation points, not a rewrite.
- Q: Log level configuration? -> A: Already handled by Spec 012 (config system). The observability spec focuses on metrics, not logging.

### Evolution 2026-02-20 (OTel GenAI Semantic Conventions)

Added `gen_ai.*` metrics following the [OpenTelemetry Semantic Conventions for Generative AI](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/). These are emitted alongside the `antwort_*` operational metrics for interoperability with LLM observability tooling (Datadog, Grafana, etc.).

New metrics: `gen_ai_client_token_usage`, `gen_ai_client_operation_duration_seconds`, `gen_ai_server_time_to_first_token_seconds`, `gen_ai_server_time_per_output_token_seconds`.

TTFT and time-per-output-token require timing changes in the streaming path (record first chunk timestamp, compute per-token timing).

## User Scenarios & Testing

### User Story 1 - Monitor Request Traffic (Priority: P1)

An operator monitors antwort request traffic via Prometheus. They can see total requests by method/status/model, request duration distributions, and active streaming connections. This enables alerting on error rates and latency spikes.

**Why this priority**: Request metrics are the foundation of production monitoring.

**Acceptance Scenarios**:

1. **Given** a running antwort instance, **When** `/metrics` is accessed, **Then** it returns Prometheus-formatted metrics
2. **Given** requests being served, **When** metrics are scraped, **Then** `antwort_requests_total` shows request counts by method, status, and model
3. **Given** streaming requests, **When** metrics are scraped, **Then** `antwort_streaming_connections_active` shows current connection count

---

### User Story 2 - Monitor Provider Latency (Priority: P1)

An operator monitors backend inference latency. They can see per-provider, per-model latency distributions and token throughput. This helps identify slow models and capacity bottlenecks.

**Acceptance Scenarios**:

1. **Given** inference requests, **When** metrics are scraped, **Then** `antwort_provider_latency_seconds` shows latency distribution per provider and model
2. **Given** completed requests, **When** metrics are scraped, **Then** `antwort_provider_tokens_total` shows input and output token counts

---

### User Story 3 - Monitor Tool Execution (Priority: P2)

An operator monitors MCP tool execution. They can see which tools are called, how often they succeed or fail, and execution latency.

**Acceptance Scenarios**:

1. **Given** agentic loop requests with tool calls, **When** metrics are scraped, **Then** `antwort_tool_executions_total` shows counts by tool name and status

---

### Edge Cases

- What happens when the metrics endpoint is accessed without Prometheus? It returns plain text in Prometheus exposition format, readable by any HTTP client.
- What happens when metrics cardinality is too high (many models)? Model names are used as-is. Operators should limit model diversity or use relabeling.
- What happens when the server starts with metrics disabled? The `/metrics` endpoint returns 404. No metrics are collected.

## Requirements

### Functional Requirements

**Metrics Endpoint**

- **FR-001**: The server MUST expose a `/metrics` endpoint returning Prometheus-formatted metrics
- **FR-002**: The metrics endpoint MUST be accessible without authentication (same bypass as /healthz)
- **FR-003**: Metrics collection MUST be configurable (enabled by default, can be disabled)

**Request Metrics**

- **FR-004**: The system MUST record `antwort_requests_total` counter with labels: method (POST/GET/DELETE), status (2xx/4xx/5xx), model
- **FR-005**: The system MUST record `antwort_request_duration_seconds` histogram with LLM-tuned buckets
- **FR-006**: The system MUST record `antwort_streaming_connections_active` gauge for current streaming connections

**Provider Metrics**

- **FR-007**: The system MUST record `antwort_provider_requests_total` counter with labels: provider, model, status
- **FR-008**: The system MUST record `antwort_provider_latency_seconds` histogram with LLM-tuned buckets
- **FR-009**: The system MUST record `antwort_provider_tokens_total` counter with labels: provider, model, direction (input/output)

**Tool Metrics**

- **FR-010**: The system MUST record `antwort_tool_executions_total` counter with labels: tool_name, status (success/error)

**Rate Limiting Metrics**

- **FR-011**: The system MUST record `antwort_ratelimit_rejected_total` counter with label: tier

**OpenTelemetry GenAI Semantic Conventions (gen_ai.*)**

The system also exposes metrics following the [OTel GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/) for interoperability with LLM observability tooling. These are emitted alongside the `antwort_*` operational metrics.

- **FR-012a**: The system MUST record `gen_ai_client_token_usage` histogram with attributes: `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.token.type` (input/output), `gen_ai.request.model`, `gen_ai.response.model`
- **FR-012b**: The system MUST record `gen_ai_client_operation_duration_seconds` histogram with attributes: `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.response.model`, `error.type`
- **FR-012c**: The system MUST record `gen_ai_server_time_to_first_token_seconds` histogram for streaming requests, measuring the time from request start to the first content token received from the backend
- **FR-012d**: The system MUST record `gen_ai_server_time_per_output_token_seconds` histogram measuring decode latency: (total duration minus TTFT) divided by (output tokens minus 1)

**LLM-Tuned Buckets**

- **FR-013**: Duration histograms MUST use LLM-tuned bucket boundaries: 0.1, 0.5, 1, 2, 5, 10, 30, 60, 120 seconds
- **FR-013a**: Token histograms MUST use token-count bucket boundaries: 1, 4, 16, 64, 256, 1024, 4096, 16384

**Configuration**

- **FR-014**: Metrics MUST be configurable via the config system (Spec 012): enabled/disabled, endpoint path

### Key Entities

- **Metrics Registry**: Central registry holding all metric collectors.
- **Request Middleware**: Transport-layer middleware recording request metrics.
- **Provider Instrumentation**: Hooks in the engine recording provider latency and token counts.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The `/metrics` endpoint returns valid Prometheus exposition format
- **SC-002**: Request count and duration metrics are recorded for every request
- **SC-003**: Provider latency and token metrics are recorded for every inference call
- **SC-004**: Metrics are scrapable by Prometheus via the existing ServiceMonitor
- **SC-005**: The `gen_ai.*` metrics follow OTel GenAI semantic conventions and include TTFT and time-per-output-token for streaming requests

## Assumptions

- The Prometheus client library is an external dependency in the observability package.
- Metrics are recorded synchronously in the request path (negligible overhead).
- OpenTelemetry tracing is deferred to a separate spec.
- The ServiceMonitor in the OpenShift overlay (Spec 010) already targets the metrics port.

## Dependencies

- **Spec 010 (Kustomize Deploy)**: ServiceMonitor for Prometheus scraping.
- **Spec 012 (Configuration)**: Metrics on/off config.

## Scope Boundaries

### In Scope

- Prometheus metrics (counters, histograms, gauges)
- `/metrics` endpoint
- LLM-tuned histogram buckets
- Request, provider, tool, rate limiting metrics
- OTel GenAI semantic convention metrics (gen_ai.*)
- TTFT and time-per-output-token for streaming
- Config integration (enabled/disabled)

### Out of Scope

- OpenTelemetry distributed tracing (future spec)
- Grafana dashboards
- Custom alerting rules
- Log aggregation
