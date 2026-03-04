# Feature Specification: Audit Logging

**Feature Branch**: `042-audit-logging`
**Created**: 2026-03-04
**Status**: Draft
**Input**: Brainstorm 34: Audit logging for compliance, debugging, and operational visibility in multi-user deployments

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Debug Authorization Issues (Priority: P1)

An operator managing a multi-user antwort deployment receives a report from a user ("I can't see my vector store"). The operator checks the audit log and finds an ownership denial event showing the user's identity, the resource they tried to access, and the operation they attempted. The operator quickly identifies whether the issue is a misconfigured role, a missing scope, or a legitimate ownership boundary.

**Why this priority**: This is the primary motivation for audit logging. Without an observable authorization trail, debugging "why can't I access X?" questions in multi-user deployments requires guessing and reproducing conditions manually.

**Independent Test**: Can be fully tested by enabling audit logging, making requests as different users with different permission levels, and verifying that the audit log contains correct event records with the right identity and resource details.

**Acceptance Scenarios**:

1. **Given** audit logging is enabled and a user attempts to access a resource they do not own, **When** the request is processed, **Then** the audit log contains an ownership denial event with the user's identity, the resource type, the resource ID, and the operation attempted.
2. **Given** audit logging is enabled and a user attempts an action they lack scope for, **When** the request is rejected with 403, **Then** the audit log contains a scope denial event with the user's identity, the endpoint, the required scope, and the user's effective scopes.
3. **Given** audit logging is enabled and an admin accesses another user's resource, **When** the request succeeds via admin override, **Then** the audit log contains an admin override event identifying both the admin and the resource owner.

---

### User Story 2 - Track Resource Mutations (Priority: P2)

An operator needs to understand who created or deleted resources in a shared deployment. When a vector store or file disappears unexpectedly, the operator checks the audit log and finds the deletion event with the identity of the user who performed it.

**Why this priority**: Mutation tracking is essential for accountability in shared environments but is secondary to authorization debugging, which blocks day-to-day operations.

**Independent Test**: Can be fully tested by enabling audit logging, creating and deleting resources as different users, and verifying that the audit log records each mutation with the correct identity and resource details.

**Acceptance Scenarios**:

1. **Given** audit logging is enabled and a user creates a resource, **When** the resource is successfully persisted, **Then** the audit log contains a creation event with the user's identity, the resource type, and the resource ID.
2. **Given** audit logging is enabled and a user deletes a resource, **When** the resource is successfully removed, **Then** the audit log contains a deletion event with the user's identity, the resource type, and the resource ID.
3. **Given** audit logging is enabled and a user changes permissions on a resource, **When** the permissions update succeeds, **Then** the audit log contains a permissions-changed event showing the old and new permission values.

---

### User Story 3 - Monitor Authentication Activity (Priority: P2)

A security-conscious operator wants visibility into authentication attempts across the deployment. They check the audit log to see successful and failed authentication events, identifying potential brute-force attempts or misconfigured clients.

**Why this priority**: Authentication monitoring is important for security but is secondary to authorization debugging, which addresses the most common operational question ("why can't user X do Y?").

**Independent Test**: Can be fully tested by enabling audit logging, sending requests with valid and invalid credentials, and verifying that the audit log records each authentication outcome.

**Acceptance Scenarios**:

1. **Given** audit logging is enabled and a user authenticates successfully, **When** the request is processed, **Then** the audit log contains a success event with the user's identity, the authentication method used, and the client's network address.
2. **Given** audit logging is enabled and a request fails authentication, **When** the request is rejected, **Then** the audit log contains a failure event with the authentication method attempted, the client's network address, and the reason for failure.

---

### User Story 4 - Track Tool Execution (Priority: P3)

An operator monitoring an agentic deployment wants to see which tools are being invoked and whether any are failing. They check the audit log to find tool execution events tied to specific users and responses.

**Why this priority**: Tool execution visibility is valuable for debugging agentic workflows but is lower priority than core authorization and mutation tracking.

**Independent Test**: Can be fully tested by enabling audit logging, triggering tool calls through the agentic loop, and verifying that the audit log records each tool dispatch and any failures.

**Acceptance Scenarios**:

1. **Given** audit logging is enabled and the engine dispatches a tool call, **When** the tool executes, **Then** the audit log contains an execution event with the user's identity, the tool type, the tool name, and the associated response ID.
2. **Given** audit logging is enabled and a tool execution fails, **When** the failure is handled, **Then** the audit log contains a failure event with the error details in addition to the tool identification fields.

---

### User Story 5 - Opt-In with Zero Overhead (Priority: P1)

An operator running a minimal development deployment does not enable audit logging. The system starts normally with no audit-related overhead, no configuration required, and no audit output. A production operator enables audit logging via configuration and receives structured audit events without restarting the system's core logic.

**Why this priority**: Antwort's nil-safe composition principle requires that optional capabilities add zero overhead when disabled. Audit logging must follow this pattern.

**Independent Test**: Can be fully tested by starting antwort without audit configuration and verifying no audit output, then starting with audit enabled and verifying events appear.

**Acceptance Scenarios**:

1. **Given** audit logging is not configured, **When** the server starts and processes requests, **Then** no audit events are emitted and no errors occur related to audit.
2. **Given** audit logging is enabled in configuration, **When** the server starts, **Then** the audit log contains a startup event recording the security configuration state.
3. **Given** audit logging is enabled, **When** audit events are emitted, **Then** each event includes a timestamp, event name, and severity level in a structured format.

---

### Edge Cases

- What happens when audit logging is enabled but the output destination is unavailable (e.g., file path not writable)? The system should report the configuration error at startup and fail to start, not silently drop events.
- What happens when a request has no authenticated identity (e.g., NoOp authenticator)? Audit events should still be recorded with the identity fields empty or marked as anonymous.
- What happens when multiple audit events occur for a single request (e.g., auth success followed by scope denial)? Each event is recorded independently with its own timestamp.
- What happens during server shutdown? In-flight audit events should be flushed before the process exits.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a dedicated audit event channel, separate from operational logging, for recording security-relevant events and state-changing operations.
- **FR-002**: System MUST record authentication events: successful authentication, failed authentication, and rate limit enforcement.
- **FR-003**: System MUST record authorization events: scope denial (403), ownership denial (404-equivalent), and admin override usage.
- **FR-004**: System MUST record resource mutation events: resource creation, resource deletion, and permission changes.
- **FR-005**: System MUST record tool execution events: tool dispatch and tool failure.
- **FR-006**: System MUST record a startup event capturing the security configuration state (authentication enabled, audit enabled, role count, scope enforcement status).
- **FR-007**: Each audit event MUST automatically include the authenticated user's identity and tenant membership when available from the request context.
- **FR-008**: Each audit event MUST include a timestamp, event name, and severity level.
- **FR-009**: Audit events MUST be output in a structured format suitable for machine parsing and log aggregation tools.
- **FR-010**: Audit logging MUST be disabled by default, adding zero overhead when not configured (nil-safe composition).
- **FR-011**: Audit logging MUST be configurable for output format and output destination.
- **FR-012**: System MUST NOT audit read operations (only security events and mutations are in scope to manage event volume).

### Key Entities

- **Audit Event**: A structured record of a security-relevant occurrence. Contains: timestamp, event name, severity, and event-specific fields. Optionally includes: user identity, tenant membership, and resource details.
- **Audit Event Catalog**: The defined set of audit event types, organized into categories: authentication (3 events), authorization (3 events), resource mutation (3 events), tool execution (2 events), and system (1 event). Total: 12 event types.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can determine the cause of any authorization denial within 60 seconds by filtering audit events for a specific user and time range.
- **SC-002**: All 12 defined audit event types are recorded at the correct integration points when audit logging is enabled.
- **SC-003**: With audit logging disabled, the system shows no measurable performance difference compared to a build without audit code (zero overhead when off).
- **SC-004**: Audit events contain sufficient context to answer "who did what, to which resource, when, and whether it was allowed" without correlating with other log sources.
- **SC-005**: Audit output integrates with standard log aggregation pipelines (e.g., fluentd, vector) without custom parsing configuration, using structured format.

## Assumptions

- Phase 1 focuses on structured audit logging to stdout or file. Compliance extensions (tamper-evident storage, event signing, retention policies, query API) are explicitly deferred to a future Phase 2 specification.
- Read operations are excluded from auditing. This decision manages event volume while still capturing all security-relevant events and state changes.
- The audit event catalog of 12 events covers the integration points identified in the existing codebase (authentication middleware, scope middleware, storage layer, HTTP handlers, engine loop).
- Log shipping and aggregation are infrastructure concerns handled by external tools (fluentd, vector, etc.), not by antwort itself.

## Dependencies

- **Spec 007 (Authentication)**: Provides the identity context that audit events extract automatically.
- **Spec 040 (Resource Ownership)**: Provides ownership context and admin override paths that generate authorization audit events.
- **Spec 041 (Scope Permissions)**: Provides scope enforcement middleware that generates scope denial audit events.
