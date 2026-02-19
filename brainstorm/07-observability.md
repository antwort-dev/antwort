# Spec 07d: Observability

**Branch**: `spec/07d-observability`
**Dependencies**: Spec 03 (Core Engine), Spec 07b (Kustomize)
**Package**: `pkg/observability`

## Purpose

Add Prometheus metrics, structured logging configuration, and OpenTelemetry tracing to the antwort server.

## Scope

### In Scope
- Prometheus metrics with LLM-tuned histogram buckets
- Request log field reference (structured slog fields)
- OpenTelemetry distributed tracing with configurable sampling
- Trace span hierarchy (request -> provider -> tools -> storage)
- Metrics endpoint (/metrics)
- ServiceMonitor for Prometheus scraping

### Out of Scope
- Grafana dashboards (future)
- Log aggregation infrastructure (external)
- Custom alerting rules (operational concern)

## Key Metrics

```
antwort_requests_total{method, status, model}
antwort_request_duration_seconds{method, model}
antwort_streaming_connections_active{model}
antwort_provider_requests_total{provider, model, status}
antwort_provider_latency_seconds{provider, model}
antwort_provider_tokens_total{provider, model, direction}
antwort_tool_executions_total{tool_name, status}
antwort_ratelimit_rejected_total{tier}
```

LLM-tuned histogram buckets: `{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}`

## Deliverables

- [ ] `pkg/observability/metrics.go` - Prometheus metrics
- [ ] `pkg/observability/tracing.go` - OpenTelemetry setup
- [ ] Server integration (metrics endpoint, trace middleware)
