# Feature Specification: Sandbox Server for Code Execution

**Feature Branch**: `024-sandbox-server`
**Created**: 2026-02-25
**Status**: Draft

## Overview

Antwort's constitution (Principle IX) states that all tool code execution is delegated to isolated sandbox pods. This specification delivers the sandbox server: a lightweight HTTP service that runs inside agent-sandbox pods and executes Python code on demand. It also delivers the container image that packages the server with a Python runtime.

The sandbox server is the execution backend for the `code_interpreter` tool (Spec 025). It receives code via a REST API, executes it in an isolated subprocess with timeout enforcement, and returns stdout, stderr, and exit code. The container image is an antwort project deliverable; the pod lifecycle is managed by the agent-sandbox controller (Kubernetes SIG).

## Clarifications

### Session 2026-02-25

- Q: Should the sandbox server be written in Go or Python? A: Go, for consistency with the rest of the project. It shells out to Python for code execution.
- Q: Should the server support package installation? A: Yes, via `uv pip install` before execution. The caller specifies requirements in the request.
- Q: Should the server support file I/O? A: Yes, the caller can send files (base64-encoded) and receive files produced by the code.

## User Scenarios & Testing

### User Story 1 - Execute Python Code (Priority: P1)

An agentic gateway sends a code execution request to a sandbox pod. The sandbox server receives the code, executes it in an isolated Python subprocess with a timeout, and returns the output. The code runs as an unprivileged user with no network access (enforced by NetworkPolicy, not by the server).

**Why this priority**: This is the core purpose of the sandbox server. Without it, the `code_interpreter` tool has no backend.

**Independent Test**: Send a code execution request to the sandbox server, verify stdout is returned correctly.

**Acceptance Scenarios**:

1. **Given** a running sandbox server, **When** a code execution request is sent with Python code, **Then** the server returns stdout, stderr, and exit code
2. **Given** a code execution request with a timeout, **When** the code exceeds the timeout, **Then** the server terminates the subprocess and returns an error with exit code -1
3. **Given** a code execution request with package requirements, **When** the server receives it, **Then** it installs the packages before executing the code
4. **Given** a code execution request that produces output files, **When** the code writes to a designated output directory, **Then** the server returns the files in the response

---

### User Story 2 - Health and Status Reporting (Priority: P1)

The sandbox controller and the gateway need to know if the sandbox pod is healthy and available. The sandbox server exposes a health endpoint that reports its status and current load.

**Why this priority**: Required for Kubernetes readiness probes and for the gateway to know when a sandbox is available.

**Independent Test**: Query the health endpoint, verify it returns status and capacity information.

**Acceptance Scenarios**:

1. **Given** a running sandbox server, **When** the health endpoint is queried, **Then** it returns "healthy" status with capacity and current load
2. **Given** a sandbox server currently executing code, **When** the health endpoint is queried, **Then** it reports the current load accurately

---

### User Story 3 - Container Image (Priority: P1)

An operator deploys agent-sandbox with a SandboxTemplate that references the antwort sandbox container image. The image contains the sandbox server binary, Python runtime, and the `uv` package manager. The image starts the sandbox server on port 8080.

**Why this priority**: The container image is needed for any sandbox deployment.

**Independent Test**: Build the container image, start it, send a code execution request.

**Acceptance Scenarios**:

1. **Given** the sandbox container image, **When** started as a container, **Then** the sandbox server listens on port 8080 and responds to health checks
2. **Given** the sandbox container image, **When** a code execution request is sent, **Then** Python code executes successfully and the result is returned

---

### Edge Cases

- What happens when the Python subprocess crashes? The server catches the error and returns stderr with a non-zero exit code.
- What happens when the server receives concurrent execution requests? Each request runs in its own subprocess. The server reports current load via the health endpoint.
- What happens when package installation fails? The server returns an error before attempting code execution, with the pip/uv error output in stderr.
- What happens when the code tries to access the network? Network access is controlled by Kubernetes NetworkPolicy, not by the sandbox server. The server does not enforce network isolation.
- What happens when disk space is exhausted? The subprocess fails, and the error is returned in stderr.

## Requirements

### Functional Requirements

**Code Execution**

- **FR-001**: The sandbox server MUST accept code execution requests via a REST endpoint
- **FR-002**: The request MUST support a `code` field (Python source code), a `timeout_seconds` field, and an optional `requirements` field (list of pip packages)
- **FR-003**: The server MUST execute the code in a separate subprocess, not in the server process
- **FR-004**: The server MUST enforce the timeout by terminating the subprocess if it exceeds the specified duration
- **FR-005**: The response MUST include `stdout`, `stderr`, `exit_code`, and `execution_time_ms`
- **FR-006**: The request MAY include an `files` field (map of filename to base64-encoded content) that are written to the working directory before execution
- **FR-007**: The response MAY include a `files_produced` field containing files written to a designated output directory during execution

**Package Management**

- **FR-008**: When `requirements` are specified, the server MUST install them using a package manager before executing the code
- **FR-009**: The server MUST support configurable package index URLs (for private registries and air-gapped environments)

**Health and Status**

- **FR-010**: The server MUST expose a health endpoint that returns status, capacity, and current load
- **FR-011**: The health endpoint MUST be usable as a Kubernetes readiness probe

**Container Image**

- **FR-012**: The project MUST produce a container image containing the sandbox server binary and a Python runtime
- **FR-013**: The container image MUST include the `uv` package manager for fast package installation
- **FR-014**: The container image MUST run as a non-root user
- **FR-015**: The container image MUST expose port 8080

**Security**

- **FR-016**: The sandbox server MUST NOT execute code as root
- **FR-017**: Code execution MUST happen in a temporary working directory that is cleaned up after each request

## Success Criteria

- **SC-001**: Python code sent to the sandbox server produces correct output and is returned to the caller
- **SC-002**: Code that exceeds the timeout is terminated and returns an error within 1 second of the deadline
- **SC-003**: The container image starts and passes health checks within 5 seconds
- **SC-004**: The sandbox server handles at least 3 concurrent execution requests without errors

## Assumptions

- The sandbox pod runs inside a Kubernetes cluster with agent-sandbox controller managing the pod lifecycle.
- Network isolation is enforced by Kubernetes NetworkPolicy, not by the sandbox server itself.
- The sandbox server does not need to manage its own TLS. mTLS (if needed) is handled by a sidecar or service mesh. For the initial version, plain HTTP within the cluster network is acceptable.
- The `uv` package manager is used instead of `pip` for faster package installation.
- The sandbox server is stateless between requests. Each request gets a fresh working directory.
- File sizes are limited by the request body size (reasonable default: 10MB).

## Dependencies

- **agent-sandbox** (kubernetes-sigs): Provides Sandbox, SandboxTemplate, SandboxWarmPool CRDs and controller for pod lifecycle management.
- **Python 3.12+**: Runtime for executing user code.
- **uv**: Fast Python package manager.

## Scope Boundaries

### In Scope

- Sandbox server binary (HTTP server accepting code execution requests)
- Container image (server + Python + uv)
- Health endpoint for readiness probes
- Code execution with timeout enforcement
- Package installation via uv
- File I/O (send files in, get files out)
- Containerfile for building the image

### Out of Scope

- Pod lifecycle management (handled by agent-sandbox controller)
- Network isolation (handled by Kubernetes NetworkPolicy)
- mTLS / SPIFFE identity (deferred, handled by service mesh or sidecar)
- Virtual environment caching across requests (deferred optimization)
- Environment routing (multiple cached envs per pod, deferred)
- The `code_interpreter` FunctionProvider in antwort (Spec 025)
- SandboxTemplate and SandboxWarmPool resource definitions (deployment concern)
