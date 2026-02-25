# Feature Specification: Sandbox Multi-Runtime Modes

**Feature Branch**: `027-sandbox-modes`
**Created**: 2026-02-25
**Status**: Draft

## Overview

The sandbox server (Spec 024) currently only executes Python code. Different agent use cases require different runtimes: data science agents need Python with heavy libraries pre-installed, DevOps agents may need shell access with CLI tools, and code generation agents may need a Go compiler or Node.js runtime. This specification adds a mode system to the sandbox server so that one binary supports multiple runtimes, selected by configuration.

The REST API contract (`POST /execute`, `GET /health`) remains identical across all modes. The mode determines which interpreter runs the code, what file extension is used, and how packages are installed. Different container images package the same binary with different runtimes installed.

## User Scenarios & Testing

### User Story 1 - Python Mode (Priority: P1)

An operator deploys a sandbox container image with Python installed. The sandbox server detects Python and uses it as the interpreter. This is the current default behavior, preserved for backward compatibility.

**Why this priority**: Python is the existing and most common mode. This story ensures no regression.

**Independent Test**: Start the sandbox server without any mode configuration, verify Python code executes correctly.

**Acceptance Scenarios**:

1. **Given** a sandbox server with Python installed and no mode configured, **When** code is submitted, **Then** it executes as Python (auto-detected)
2. **Given** a sandbox server with mode explicitly set to "python", **When** code is submitted, **Then** it executes as Python
3. **Given** Python mode, **When** requirements are specified, **Then** packages are installed before execution

---

### User Story 2 - Shell Mode (Priority: P1)

An operator deploys a sandbox with shell tools (jq, yq, curl, git). The sandbox server runs submitted code as a shell script, enabling agents to execute system commands.

**Why this priority**: Shell access is the second most common need after Python, especially for DevOps agents.

**Independent Test**: Set mode to "shell", submit a shell script, verify it executes with bash.

**Acceptance Scenarios**:

1. **Given** mode set to "shell", **When** a shell script is submitted, **Then** it executes with bash
2. **Given** shell mode, **When** the code uses common CLI tools, **Then** the tools are available (provided they're in the container image)
3. **Given** shell mode, **When** requirements are specified, **Then** the requirements field is ignored (no package installer for shell)

---

### User Story 3 - Go Mode (Priority: P2)

An operator deploys a sandbox with the Go compiler. The sandbox server compiles and runs submitted Go code.

**Why this priority**: Go is relevant for code generation and testing agents, but less common than Python or shell.

**Independent Test**: Set mode to "golang", submit Go code with a main function, verify it compiles and runs.

**Acceptance Scenarios**:

1. **Given** mode set to "golang", **When** Go source code is submitted, **Then** it is compiled and executed
2. **Given** Go mode, **When** the code has compilation errors, **Then** the compiler error appears in stderr

---

### User Story 4 - Node.js Mode (Priority: P2)

An operator deploys a sandbox with Node.js. The sandbox server runs submitted JavaScript or TypeScript code.

**Why this priority**: Node.js is common for web-related agents but less critical than Python and shell.

**Independent Test**: Set mode to "node", submit JavaScript code, verify it executes.

**Acceptance Scenarios**:

1. **Given** mode set to "node", **When** JavaScript code is submitted, **Then** it executes with Node.js
2. **Given** Node.js mode, **When** requirements are specified, **Then** packages are installed via npm before execution

---

### User Story 5 - Auto-Detection (Priority: P1)

An operator deploys a sandbox without specifying a mode. The sandbox server automatically detects which runtimes are available and selects one.

**Why this priority**: Auto-detection provides a good default experience without requiring explicit configuration.

**Independent Test**: Start without mode set, verify the server detects the available runtime and reports it in the health endpoint.

**Acceptance Scenarios**:

1. **Given** no mode configured, **When** the server starts in a container with Python, **Then** Python mode is auto-detected
2. **Given** no mode configured, **When** the server starts in a container with only Go, **Then** Go mode is auto-detected
3. **Given** no mode configured, **When** no supported runtime is found, **Then** the server fails to start with a clear error message

---

### User Story 6 - Health Reports Mode (Priority: P1)

The health endpoint reports the active mode and runtime version so that the gateway (or an operator) can verify which runtime a sandbox is using.

**Why this priority**: Without mode reporting, operators can't verify which runtime is active.

**Independent Test**: Query /health, verify the response includes the mode and runtime version.

**Acceptance Scenarios**:

1. **Given** a running sandbox in Python mode, **When** /health is queried, **Then** the response includes `"mode": "python"` and `"runtime_version": "Python 3.12.x"`
2. **Given** a running sandbox in shell mode, **When** /health is queried, **Then** the response includes `"mode": "shell"` and the bash version

---

### Edge Cases

- What happens when the configured mode doesn't match the installed runtime? The server fails to start with a clear error (e.g., "mode=golang but 'go' not found in PATH").
- What happens when code for a different language is submitted to the wrong mode? The interpreter fails (syntax error), and the error appears in stderr. The server doesn't try to detect the code's language.
- What happens when requirements are specified in shell mode? The requirements field is silently ignored.
- What happens when auto-detection finds multiple runtimes? The first detected runtime wins, in priority order: python, go, node, shell.

## Requirements

### Functional Requirements

**Mode Configuration**

- **FR-001**: The sandbox server MUST support a configurable runtime mode
- **FR-002**: The mode MUST be configurable via environment variable
- **FR-003**: Supported modes MUST include: `python`, `golang`, `node`, `shell`
- **FR-004**: When no mode is configured, the server MUST auto-detect the available runtime

**Mode Behavior**

- **FR-005**: Each mode MUST determine the interpreter command used to execute submitted code
- **FR-006**: Each mode MUST determine the file extension for the code file
- **FR-007**: Each mode MUST determine how (or whether) package requirements are installed
- **FR-008**: The REST API contract (`POST /execute`, `GET /health`) MUST remain identical across all modes

**Auto-Detection**

- **FR-009**: Auto-detection MUST check for runtimes in priority order: python, golang, node, shell
- **FR-010**: If no supported runtime is found, the server MUST fail to start with a descriptive error

**Health Reporting**

- **FR-011**: The health endpoint MUST report the active mode
- **FR-012**: The health endpoint MUST report the runtime version string

**Mode Details**

- **FR-013**: Python mode MUST execute code with `python3` and install packages with `uv pip install --target`
- **FR-014**: Shell mode MUST execute code with `bash` and skip package installation
- **FR-015**: Go mode MUST execute code with `go run` and skip package installation
- **FR-016**: Node.js mode MUST execute code with `node` and install packages with `npm install` when requirements are specified

## Success Criteria

- **SC-001**: The same sandbox server binary can execute Python, shell, Go, and Node.js code depending on the configured mode
- **SC-002**: Existing Python-mode behavior is preserved with zero regressions
- **SC-003**: The health endpoint accurately reports the active mode and runtime version
- **SC-004**: Auto-detection selects the correct runtime when no mode is configured

## Assumptions

- Each container image installs only one primary runtime (e.g., the Go image has Go but not Python). The mode selects which one to use.
- Go code is expected to include a `package main` and `func main()`. The sandbox doesn't add boilerplate.
- Node.js package installation uses npm, not yarn or pnpm.
- Shell mode uses bash. Shells like zsh or fish are not supported.
- The auto-detection priority order (python first) matches the most common deployment scenario.

## Dependencies

- **Spec 024 (Sandbox Server)**: The existing sandbox server binary being extended

## Scope Boundaries

### In Scope

- Mode configuration via environment variable
- Auto-detection of available runtimes
- Python, Go, Node.js, and shell interpreter support
- Mode-specific package installation
- Health endpoint mode/version reporting
- Unit tests for mode selection and auto-detection

### Out of Scope

- New container image Dockerfiles for each flavor (deployment concern, separate from the binary)
- Mode selection by the gateway (the gateway calls a sandbox at a known URL, it doesn't pick the mode)
- Runtime-specific features (e.g., Go module support, Node.js ESM vs CJS)
- Warm pool per mode (agent-sandbox concern)
