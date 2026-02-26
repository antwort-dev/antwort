# Data Model: Code Interpreter Tool

## Existing Types (already implemented)

### CodeInterpreterCallData (`pkg/api/types.go`)

```go
type CodeInterpreterCallData struct {
    Code    string                    `json:"code"`
    Outputs []CodeInterpreterOutput   `json:"outputs"`
}
```

### CodeInterpreterOutput (`pkg/api/types.go`)

```go
type CodeInterpreterOutput struct {
    Type  string                        `json:"type"` // "logs" or "image"
    Logs  string                        `json:"logs,omitempty"`
    Image *CodeInterpreterOutputImage   `json:"image,omitempty"`
}
```

### CodeInterpreterOutputImage (`pkg/api/types.go`)

```go
type CodeInterpreterOutputImage struct {
    FileID string `json:"file_id"`
    URL    string `json:"url,omitempty"`
}
```

### SandboxRequest (`pkg/tools/builtins/codeinterpreter/types.go`)

```go
type SandboxRequest struct {
    Code           string            `json:"code"`
    TimeoutSeconds int               `json:"timeout_seconds"`
    Requirements   []string          `json:"requirements,omitempty"`
    Files          map[string]string `json:"files,omitempty"`
}
```

### SandboxResponse (`pkg/tools/builtins/codeinterpreter/types.go`)

```go
type SandboxResponse struct {
    Status          string            `json:"status"`
    Stdout          string            `json:"stdout"`
    Stderr          string            `json:"stderr"`
    ExitCode        int               `json:"exit_code"`
    ExecutionTimeMs int64             `json:"execution_time_ms"`
    FilesProduced   map[string]string `json:"files_produced,omitempty"`
}
```

## New Types (this plan)

### SandboxClaim Adapter Types (`pkg/tools/builtins/codeinterpreter/kubernetes/`)

No new domain types needed. The adapter uses agent-sandbox API types directly:

- `extensionsv1alpha1.SandboxClaim` for creating/deleting claims
- `sandboxv1alpha1.Sandbox` for watching readiness and reading `serviceFQDN`

### claimAcquirer (`pkg/tools/builtins/codeinterpreter/kubernetes/acquirer.go`)

Implements the existing `SandboxAcquirer` interface:

```go
type claimAcquirer struct {
    client    client.Client       // controller-runtime client
    template  string              // SandboxTemplate name
    namespace string              // namespace for claims
    timeout   time.Duration       // how long to wait for Ready
}
```

**State transitions** (managed by agent-sandbox controller, observed by adapter):

```
SandboxClaim created
    -> Sandbox created (same name, owned by claim)
        -> Sandbox condition Ready=True (pod assigned, serviceFQDN populated)
            -> Adapter reads serviceFQDN, returns to caller
                -> Execution completes
                    -> SandboxClaim deleted (cascades to Sandbox)
```

## Relationships

```
CodeInterpreterProvider
    |-- SandboxAcquirer (interface)
    |       |-- staticURLAcquirer (existing, dev mode)
    |       |-- claimAcquirer (new, production mode)
    |-- SandboxClient (existing, HTTP to sandbox server)
```
