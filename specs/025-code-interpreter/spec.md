# Feature Specification: Code Interpreter Tool

**Feature Branch**: `025-code-interpreter`
**Created**: 2026-02-25
**Status**: Draft

## Overview

The gateway can execute tool calls during the agentic loop, but has no built-in capability to run arbitrary code. This specification adds a `code_interpreter` tool that executes Python code in isolated sandbox pods managed by the agent-sandbox Kubernetes controller. When a model decides it needs to run code (data analysis, calculations, file processing), the gateway acquires a sandbox pod, sends the code, collects results, and returns them to the agentic loop.

The code interpreter uses the sandbox server (Spec 024) as its execution backend and follows the upstream OpenResponses `code_interpreter_call` item format for output.

## Clarifications

### Session 2026-02-25

- Q: SandboxClaim per execution or static URL? A: SandboxClaim per execution for true pod-level isolation. Static URL as a fallback for development/testing.
- Q: Output format? A: Full OpenResponses `code_interpreter_call` format with logs and image outputs.
- Q: Where does client-go go? A: Adapter package per constitution Principle II.

## User Scenarios & Testing

### User Story 1 - Code Execution in Agentic Loop (Priority: P1)

A developer sends a streaming request to a model with `code_interpreter` enabled. The model writes Python code to analyze data. The gateway acquires a sandbox pod, executes the code, and feeds the result back to the model. The client sees SSE lifecycle events showing execution progress.

**Why this priority**: This is the core differentiator. No other OpenResponses gateway offers secure code execution within the agentic loop.

**Independent Test**: Send a request with code_interpreter tool enabled, verify the model can call it and receive results.

**Acceptance Scenarios**:

1. **Given** a request with code_interpreter tool, **When** the model calls code_interpreter with Python code, **Then** the code executes in a sandbox pod and stdout is returned as a tool result
2. **Given** a streaming agentic request, **When** code_interpreter executes, **Then** the client receives `code_interpreter_call.in_progress`, `code_interpreter_call.interpreting`, and `code_interpreter_call.completed` SSE events
3. **Given** code that requires packages, **When** the model specifies requirements, **Then** packages are installed before execution
4. **Given** code that exceeds the timeout, **When** the execution times out, **Then** the sandbox is cleaned up and an error result is returned to the model

---

### User Story 2 - Code Output with Files (Priority: P1)

A developer asks a model to create a chart or process data. The model uses code_interpreter to generate output files (images, CSVs). The results include both text output and file references matching the upstream code_interpreter_call format.

**Why this priority**: Data analysis and visualization are the primary code_interpreter use cases. Without file output, the tool is limited to text.

**Independent Test**: Send a request where the model generates a file, verify the file appears in the code_interpreter output.

**Acceptance Scenarios**:

1. **Given** code that writes files to the output directory, **When** execution completes, **Then** the response includes file outputs in the code_interpreter_call format
2. **Given** code that produces both stdout and files, **When** execution completes, **Then** both logs and file outputs appear in the result

---

### User Story 3 - Sandbox Pod Lifecycle (Priority: P1)

An operator configures the code_interpreter tool with a sandbox template. When code executes, the gateway acquires a pod from the agent-sandbox warm pool, uses it for execution, then releases it. The pod returns to the warm pool for reuse.

**Why this priority**: Pod lifecycle management is essential for production use. Without it, pods leak or pool exhaustion blocks execution.

**Independent Test**: Execute code, verify the SandboxClaim is created and deleted, and the pod returns to the pool.

**Acceptance Scenarios**:

1. **Given** a configured sandbox template and warm pool, **When** code_interpreter executes, **Then** a SandboxClaim is created to acquire a pod
2. **Given** an active SandboxClaim, **When** execution completes (success or failure), **Then** the SandboxClaim is deleted and the pod returns to the warm pool
3. **Given** an exhausted warm pool, **When** code_interpreter is called, **Then** the system waits up to the claim timeout and returns an error if no pod becomes available

---

### User Story 4 - Development Mode (Priority: P2)

A developer testing locally configures a static sandbox URL instead of using SandboxClaims. The code_interpreter tool connects directly to a sandbox server without requiring the agent-sandbox controller.

**Why this priority**: Development and testing must work without the full Kubernetes infrastructure.

**Independent Test**: Configure a static sandbox URL, verify code execution works without SandboxClaims.

**Acceptance Scenarios**:

1. **Given** a static sandbox URL in configuration, **When** code_interpreter executes, **Then** the code runs against the static URL without creating SandboxClaims
2. **Given** both static URL and sandbox template configured, **Then** an error is returned at startup (mutually exclusive)

---

### Edge Cases

- What happens when the sandbox pod crashes during execution? The HTTP call times out, the SandboxClaim is deleted, and an error result is returned to the model.
- What happens when the model calls code_interpreter in a non-streaming request? The tool executes normally, but no SSE lifecycle events are emitted (same as any tool in non-streaming mode).
- What happens when code_interpreter is not configured? It is not registered as a tool and doesn't appear in tool definitions.
- What happens when multiple tool calls include code_interpreter in the same turn? Each call gets its own SandboxClaim and pod (parallel execution).

## Requirements

### Functional Requirements

**Tool Registration**

- **FR-001**: The system MUST register a `code_interpreter` tool when the code_interpreter provider is enabled in configuration
- **FR-002**: The tool definition MUST include parameters for `code` (required) and `requirements` (optional list of packages)

**Code Execution**

- **FR-003**: When the model calls code_interpreter, the system MUST execute the provided code in an isolated sandbox pod
- **FR-004**: The system MUST forward the `requirements` parameter to the sandbox for package installation before execution
- **FR-005**: The system MUST enforce a configurable execution timeout
- **FR-006**: The system MUST return stdout, stderr, and exit code from the sandbox as the tool result

**Output Format**

- **FR-007**: The tool result MUST be formatted as a `code_interpreter_call` item type matching the upstream OpenResponses specification
- **FR-008**: Text output MUST appear as `logs` type in the outputs array
- **FR-009**: Files produced during execution MUST appear as file references in the outputs array

**Sandbox Pod Lifecycle**

- **FR-010**: When configured with a sandbox template, the system MUST create a SandboxClaim to acquire a pod from the warm pool
- **FR-011**: The system MUST wait for the SandboxClaim to become ready with a configurable timeout
- **FR-012**: The system MUST delete the SandboxClaim after execution completes (success or failure)
- **FR-013**: When configured with a static URL, the system MUST call the URL directly without creating SandboxClaims

**SSE Events**

- **FR-014**: During streaming execution, the system MUST emit `code_interpreter_call.in_progress`, `code_interpreter_call.interpreting`, and `code_interpreter_call.completed` or `code_interpreter_call.failed` events

**Configuration**

- **FR-015**: The code_interpreter provider MUST be configurable via the standard provider configuration in config.yaml
- **FR-016**: The configuration MUST support either `sandbox_template` (SandboxClaim mode) or `sandbox_url` (static URL mode), not both

## Success Criteria

- **SC-001**: A model can call code_interpreter, execute Python code in a sandbox pod, and receive results within the agentic loop
- **SC-002**: File outputs from code execution appear in the response as code_interpreter_call items
- **SC-003**: Sandbox pods are acquired and released correctly, with no pod or claim leaks after 100 consecutive executions
- **SC-004**: The code_interpreter tool works in both streaming (with SSE events) and non-streaming modes
- **SC-005**: Development mode (static URL) works without the agent-sandbox controller

## Assumptions

- The agent-sandbox controller (kubernetes-sigs) is installed on the cluster for SandboxClaim mode.
- SandboxTemplate and SandboxWarmPool resources are pre-configured by the operator.
- The sandbox pods run the sandbox server from Spec 024 (same REST API contract).
- The Kubernetes API client dependency goes in an adapter package, not in core packages.
- File outputs are returned inline (base64-encoded) in the first version. A file storage service for persistent file_ids is a future concern.
- The code_interpreter tool is registered via the existing FunctionProvider interface (Spec 016) and benefits from the same auth and metrics wrapping.

## Dependencies

- **Spec 024 (Sandbox Server)**: The execution backend running in sandbox pods
- **Spec 016 (Function Registry)**: FunctionProvider interface for tool registration
- **Spec 023 (Tool Lifecycle Events)**: SSE event emission during execution
- **agent-sandbox** (kubernetes-sigs): SandboxClaim CRD and controller

## Scope Boundaries

### In Scope

- CodeInterpreter FunctionProvider implementation
- SandboxClaim client for acquiring/releasing sandbox pods
- Static URL fallback for development
- code_interpreter_call item type in the response
- File output handling (base64 inline)
- SSE lifecycle events during execution
- Configuration in config.yaml
- Integration tests with mock sandbox server

### Out of Scope

- Sandbox pod container image (Spec 024, already done)
- Sandbox flavors and mode selection (brainstorm 23, future spec)
- SPIFFE/SPIRE mTLS between gateway and sandbox (deferred)
- Persistent file storage service (future)
- Agent profiles referencing sandbox templates (brainstorm 24, future spec)
- Virtual environment caching across executions (optimization, deferred)
