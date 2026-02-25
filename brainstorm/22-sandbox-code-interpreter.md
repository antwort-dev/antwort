# Brainstorm 22: Sandbox Code Interpreter Provider

## Context

Brainstorm 11 defines the full sandbox architecture (agent-sandbox CRDs, REST API, SPIFFE, warm pools). Spec 024 delivers the sandbox server binary and container image (deployed and tested on ROSA). This brainstorm defines the FunctionProvider that connects antwort's agentic loop to sandbox pods.

This is a prerequisite for the Agent feature (brainstorm 21), where agent definitions can include `code_interpreter` as a tool type.

## Decisions

### SandboxClaim Per Execution

Each code execution creates a `SandboxClaim` CR, waits for agent-sandbox to assign a pod from the warm pool, executes code against the pod, then deletes the claim. True pod-level isolation per execution.

This requires:
- `client-go` dependency in an adapter package (per constitution Principle II)
- agent-sandbox controller installed on the cluster
- SandboxTemplate and SandboxWarmPool resources pre-configured

### Full OpenResponses Code Interpreter Format

The output matches the upstream `code_interpreter_call` item type:

```json
{
  "type": "code_interpreter_call",
  "id": "ci_abc123",
  "status": "completed",
  "code_interpreter": {
    "code": "import pandas as pd\n...",
    "outputs": [
      {"type": "logs", "logs": "Analysis complete\n42"},
      {"type": "image", "image": {"file_id": "file_xyz"}}
    ]
  }
}
```

Files produced by the code are stored (uploaded to storage or served via URL) and returned as `image` outputs with `file_id` references.

## What Gets Built

### 1. SandboxClaim Client (`pkg/tools/builtins/codeinterpreter/kubernetes/`)

An adapter package that wraps `client-go` operations:

```
CreateClaim(ctx, template, namespace, timeout) -> (podAddress, claimName, error)
DeleteClaim(ctx, claimName, namespace) -> error
```

The `CreateClaim` function:
1. Creates a `SandboxClaim` CR referencing the template
2. Watches the claim's status until it becomes Ready (or times out)
3. Extracts the pod's service address from the status
4. Returns the address for the HTTP client to use

The `DeleteClaim` function:
1. Deletes the `SandboxClaim` CR
2. Agent-sandbox controller returns the pod to the warm pool

### 2. Sandbox HTTP Client (`pkg/tools/builtins/codeinterpreter/`)

A client that calls the sandbox server's REST API:

```
Execute(ctx, podAddress, code, requirements, files, timeout) -> (ExecuteResult, error)
```

Maps to `POST /execute` on the sandbox server. Returns stdout, stderr, exit code, produced files.

### 3. CodeInterpreter FunctionProvider (`pkg/tools/builtins/codeinterpreter/`)

Implements the `FunctionProvider` interface (from the function registry, Spec 016):

```
Name() -> "code_interpreter"
Tools() -> [ToolDefinition for code_interpreter]
Execute(ctx, ToolCall) -> (ToolResult, error)
Routes() -> nil (no HTTP management routes)
```

The Execute flow:
1. Parse the tool call arguments (code, requirements)
2. Create a SandboxClaim -> get pod address
3. Call the sandbox server's /execute endpoint
4. Format the result as code_interpreter_call output
5. Delete the SandboxClaim
6. Return the formatted result

### 4. Code Interpreter Item Types (`pkg/api/types.go`)

New item types matching the upstream spec:

```go
const (
    ItemTypeCodeInterpreterCall = "code_interpreter_call"
)

type CodeInterpreterCallData struct {
    Code    string                      `json:"code"`
    Outputs []CodeInterpreterOutput     `json:"outputs"`
}

type CodeInterpreterOutput struct {
    Type  string                       `json:"type"` // "logs" or "image"
    Logs  string                       `json:"logs,omitempty"`
    Image *CodeInterpreterOutputImage  `json:"image,omitempty"`
}

type CodeInterpreterOutputImage struct {
    FileID string `json:"file_id"`
    URL    string `json:"url,omitempty"`
}
```

### 5. SSE Event Classification

Update `classifyToolType()` in `pkg/engine/loop.go` to recognize `code_interpreter`:

```go
case tools.ToolKindBuiltin:
    if toolName == "code_interpreter" {
        return "code_interpreter"
    }
```

And add the event type mapping in `toolLifecycleEvents()`:

```go
case "code_interpreter":
    return EventCodeInterpreterInProgress, EventCodeInterpreterInterpreting,
           EventCodeInterpreterCompleted, ""
```

## Configuration

```yaml
providers:
  code_interpreter:
    enabled: true
    settings:
      sandbox_template: antwort-python       # SandboxTemplate name
      sandbox_namespace: antwort              # Namespace for SandboxClaims
      claim_timeout: 30s                      # Time to wait for pod assignment
      execution_timeout: 60s                  # Default code execution timeout
      max_output_size: 10485760               # 10MB max output
```

## Fallback: Static URL Mode

For development and testing without agent-sandbox controller, support a static URL mode:

```yaml
providers:
  code_interpreter:
    enabled: true
    settings:
      sandbox_url: http://sandbox-server:8080  # Static URL (no SandboxClaim)
```

When `sandbox_url` is set, skip the SandboxClaim flow and call the URL directly. When `sandbox_template` is set, use the SandboxClaim flow. Only one should be configured.

## File Handling

Files produced by code execution are returned by the sandbox server as base64-encoded strings. The FunctionProvider:

1. Decodes the base64 content
2. Determines the file type (from extension or magic bytes)
3. For images (png, jpg, svg): creates a `CodeInterpreterOutputImage` with a file_id
4. For other files: includes them as base64 in the output or stores them

**File storage** for the first version: return files inline as base64 in the tool output. The model and client get the data directly. A file storage service (for persistent file_ids and URLs) is a future concern.

## Dependencies

- **Spec 024** (sandbox server): the execution backend
- **Spec 016** (function registry): FunctionProvider interface
- **Spec 023** (tool lifecycle events): SSE events during execution
- **agent-sandbox** (kubernetes-sigs): SandboxClaim CRD and controller
- **client-go**: For SandboxClaim CRUD (adapter package only)

## Phasing

1. Add CodeInterpreterCallData types to `pkg/api/types.go`
2. Implement sandbox HTTP client (calls /execute)
3. Implement SandboxClaim client (adapter with client-go)
4. Implement CodeInterpreter FunctionProvider
5. Update classifyToolType for code_interpreter SSE events
6. Add to config loader and server wiring
7. Integration tests (mock sandbox server, test full agentic loop)
8. End-to-end test on ROSA with real sandbox pods

## Open Questions

- Should `code_interpreter` be a standard `function_call` item or a first-class `code_interpreter_call` item? Recommendation: execute as function_call via the existing tool executor path, but format the output item as `code_interpreter_call` for spec compliance.
- How should file_ids be generated? UUID for now. A proper file storage service later.
- Should the SandboxClaim include resource limits (CPU, memory)? Yes, from the provider config or agent constraints.
- How to handle SandboxClaim timeout (agent-sandbox is slow or pool is exhausted)? Return an error tool result, don't fail the entire response.
