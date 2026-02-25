# Feature Specification: Category-Based Debug Logging

**Feature Branch**: `026-debug-logging`
**Created**: 2026-02-25
**Status**: Draft

## Overview

Debugging production issues requires visibility into the data flowing between antwort and its backends (LLM providers, MCP servers, sandbox pods). Currently the gateway logs startup information and errors, but provides no way to trace individual requests through the system. This specification adds a category-based debug logging system with two orthogonal controls: categories (which subsystems to debug) and levels (how much detail to show).

## User Scenarios & Testing

### User Story 1 - Debug Provider Communication (Priority: P1)

An operator is troubleshooting why a model produces unexpected responses. They enable debug logging for the "providers" category. The logs show the exact HTTP request sent to the LLM backend (method, URL, request body summary) and the response received (status code, timing, response body summary). This reveals whether the issue is in the request translation or the backend.

**Why this priority**: Provider communication is the most common debugging need. Everything else (tools, streaming, auth) depends on understanding what the LLM receives and returns.

**Independent Test**: Enable providers debug, send a request, verify provider request/response details appear in logs.

**Acceptance Scenarios**:

1. **Given** debug enabled for "providers", **When** a request is processed, **Then** the log shows the outbound HTTP method, URL, and a summary of the request body
2. **Given** debug enabled for "providers", **When** the backend responds, **Then** the log shows the response status, timing, and a summary of the response body
3. **Given** debug NOT enabled for "providers", **When** a request is processed, **Then** no provider debug output appears in logs (zero overhead)

---

### User Story 2 - Debug Agentic Loop (Priority: P1)

An operator is troubleshooting why the agentic loop runs too many turns or dispatches the wrong tool. They enable debug logging for the "engine" category. The logs show each turn of the loop, which tools the model requested, and how the engine dispatched them.

**Why this priority**: The agentic loop is the core orchestration layer. Understanding its decisions is essential for debugging tool call issues.

**Independent Test**: Enable engine debug, send a request with tools, verify loop turn details appear in logs.

**Acceptance Scenarios**:

1. **Given** debug enabled for "engine", **When** the agentic loop runs, **Then** the log shows each turn number, tool calls received, and dispatch decisions
2. **Given** debug enabled for "engine", **When** a tool call result is received, **Then** the log shows the tool name, success/failure, and timing

---

### User Story 3 - Multiple Categories (Priority: P1)

An operator enables multiple debug categories simultaneously to trace a request end-to-end across subsystems. They can also enable "all" to see everything.

**Why this priority**: Real debugging often requires correlating events across subsystems.

**Independent Test**: Enable multiple categories, verify output from all enabled categories appears.

**Acceptance Scenarios**:

1. **Given** debug enabled for "providers,engine", **When** a request is processed, **Then** both provider and engine debug output appears
2. **Given** debug enabled for "all", **When** a request is processed, **Then** debug output from every subsystem appears
3. **Given** no debug categories enabled, **When** a request is processed, **Then** no debug output appears (default behavior)

---

### User Story 4 - Verbosity Levels (Priority: P2)

An operator controls the overall log verbosity independently from debug categories. At DEBUG level, category output shows summaries. At TRACE level, category output includes full request/response bodies for deep protocol debugging.

**Why this priority**: Full bodies are essential for protocol-level debugging but too noisy for normal use.

**Independent Test**: Set TRACE level with providers category, verify full HTTP bodies appear in logs.

**Acceptance Scenarios**:

1. **Given** log level set to DEBUG with providers category enabled, **When** a request is processed, **Then** provider logs show summaries (method, URL, timing) but not full bodies
2. **Given** log level set to TRACE with providers category enabled, **When** a request is processed, **Then** provider logs include full request and response bodies

---

### Edge Cases

- What happens when an invalid category name is specified? It is silently ignored (no error).
- What happens when ANTWORT_LOG_LEVEL is set below DEBUG but categories are enabled? Category output is suppressed because it emits at DEBUG level.
- What happens when debug logging is enabled in production? It works but may impact performance due to string formatting. This is by design (operator's choice).
- What happens with streaming responses? Provider debug shows the stream initiation, not individual SSE chunks (which would be too verbose even at TRACE).

## Requirements

### Functional Requirements

**Category System**

- **FR-001**: The system MUST support enabling debug output for specific subsystem categories
- **FR-002**: Categories MUST be configurable via environment variable (comma-separated list)
- **FR-003**: Categories MUST also be configurable via the configuration file
- **FR-004**: The special category "all" MUST enable debug output for every subsystem
- **FR-005**: The following categories MUST be supported: `providers`, `engine`, `tools`, `sandbox`, `mcp`, `auth`, `transport`, `streaming`, `config`

**Verbosity Levels**

- **FR-006**: The system MUST support configurable log verbosity levels: ERROR, WARN, INFO, DEBUG, TRACE
- **FR-007**: The log level MUST be configurable via environment variable
- **FR-008**: The log level MUST also be configurable via the configuration file
- **FR-009**: The default log level MUST be INFO
- **FR-010**: Category debug output MUST be emitted at DEBUG level (requiring DEBUG or TRACE to be visible)
- **FR-011**: At TRACE level, debug output MUST include full untruncated request/response bodies

**Performance**

- **FR-012**: When no debug categories are enabled, the debug system MUST have zero measurable overhead (no string formatting, no allocations)
- **FR-013**: Category enablement checks MUST be constant-time operations

**Provider Category**

- **FR-014**: The "providers" category MUST log outbound HTTP requests (method, URL, body summary)
- **FR-015**: The "providers" category MUST log backend responses (status code, timing, body summary)

**Engine Category**

- **FR-016**: The "engine" category MUST log agentic loop turns (turn number, tool calls, dispatch decisions)

## Success Criteria

- **SC-001**: An operator can enable provider debugging and see the exact communication with the LLM backend within seconds of configuration change
- **SC-002**: Debug logging with no categories enabled adds zero measurable latency to request processing
- **SC-003**: All existing tests continue to pass with zero regressions
- **SC-004**: Category debug output is visible when the appropriate level and category are configured

## Assumptions

- The logging system uses the existing structured logging infrastructure (slog).
- Body summaries are truncated at a reasonable length (e.g., 1000 characters at DEBUG, unlimited at TRACE).
- Debug output includes the category name as a structured field for filtering.
- The configuration file section for logging follows the existing config patterns (YAML with env override).
- Streaming provider responses log the stream initiation and completion, not individual chunks.

## Dependencies

- **Spec 012 (Configuration)**: Config file loading and environment variable override

## Scope Boundaries

### In Scope

- Debug category system (enable/disable per subsystem)
- Verbosity level configuration (ERROR through TRACE)
- Provider category instrumentation (HTTP requests/responses)
- Engine category instrumentation (agentic loop turns)
- Configuration via environment and config file
- Zero-overhead when disabled

### Out of Scope

- Instrumentation of all categories in this spec (tools, sandbox, mcp, auth, transport, streaming, config categories are defined but instrumented incrementally in future specs)
- Log shipping or aggregation (handled by Kubernetes logging infrastructure)
- Request-scoped log correlation (e.g., trace IDs linking all logs for one request, this is an observability concern)
- Log rotation or retention (handled by container runtime)
