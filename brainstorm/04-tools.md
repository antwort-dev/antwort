# Spec 04: Tool System

**Branch**: `spec/04-tools`
**Dependencies**: Spec 01 (Core Protocol), Spec 03 (Provider)
**Package**: `github.com/rhuss/antwort/pkg/tools`

## Purpose

Implement the OpenResponses tool system including function calling, MCP integration, internally-hosted tools, and the agentic loop that orchestrates tool invocation cycles. The tool system is where antwort transitions from a simple proxy to an agentic execution engine.

## Scope

### In Scope
- Tool interface hierarchy (external vs internal)
- Function calling (externally-hosted, client-executed)
- MCP client integration (externally-hosted, server-connected)
- Internally-hosted tool interface (file search, code interpreter, custom)
- `tool_choice` and `allowed_tools` enforcement
- Agentic loop orchestration
- Tool result validation

### Out of Scope
- Specific internal tool implementations (file search indexing, sandboxed code execution) are future work, but the interface to host them is in scope
- Provider-specific tool formats (handled by Translator in Spec 03)

## Tool Interface Hierarchy

```go
// ToolKind classifies how a tool is hosted and executed.
type ToolKind int

const (
    // ToolKindFunction is an externally-hosted tool whose execution
    // is delegated back to the client.
    ToolKindFunction ToolKind = iota

    // ToolKindMCP is an externally-hosted tool connected via
    // the Model Context Protocol.
    ToolKindMCP

    // ToolKindInternal is a tool hosted and executed by antwort itself.
    ToolKindInternal
)

// ToolDefinition describes a tool available for model invocation.
type ToolDefinition struct {
    Type        string          `json:"type"` // "function", "mcp", or internal type
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    Parameters  json.RawMessage `json:"parameters,omitempty"` // JSON Schema
    Kind        ToolKind        `json:"-"`

    // For MCP tools: the server endpoint
    MCPServer   string `json:"mcp_server,omitempty"`

    // For internal tools: provider-specific config
    Config      map[string]any `json:"config,omitempty"`
}

// ToolExecutor executes a tool call and returns the result.
// Implementations differ by ToolKind.
type ToolExecutor interface {
    // Kind returns the type of tools this executor handles.
    Kind() ToolKind

    // CanExecute checks if this executor can handle the given tool.
    CanExecute(tool ToolDefinition) bool

    // Execute runs the tool and returns the result.
    // For function tools, this returns a "pending" result that
    // the client must fulfill.
    Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
}

// ToolCall represents a model's request to invoke a tool.
type ToolCall struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // JSON string
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
    CallID  string `json:"call_id"`
    Output  string `json:"output"`
    IsError bool   `json:"is_error,omitempty"`
}
```

## Tool Choice Enforcement

```go
// ToolChoiceType determines how the model selects tools.
type ToolChoiceType string

const (
    ToolChoiceAuto     ToolChoiceType = "auto"     // model decides
    ToolChoiceRequired ToolChoiceType = "required" // must call a tool
    ToolChoiceNone     ToolChoiceType = "none"     // must not call tools
)

// ToolChoice is either a string ("auto", "required", "none")
// or a structured object forcing a specific tool.
type ToolChoice struct {
    Type         ToolChoiceType `json:"type,omitempty"`
    ForcedTool   *ForcedTool    `json:"forced_tool,omitempty"`
}

type ForcedTool struct {
    Type string `json:"type"` // "function"
    Name string `json:"name"`
}

// ToolFilter validates and restricts tool access.
type ToolFilter struct {
    // AllowedTools is the subset of tools the model may invoke.
    // If empty, all tools in the request are allowed.
    AllowedTools []string

    // AllTools is the full set of tools for caching/context.
    AllTools []ToolDefinition
}

// Filter returns only the tools the model is allowed to call,
// while keeping all tools visible for context.
func (f *ToolFilter) Filter() (callable []ToolDefinition, visible []ToolDefinition)
```

## Agentic Loop

The agentic loop is the core orchestration that cycles between model inference and tool execution:

```go
// AgentLoop orchestrates the inference-tool cycle.
type AgentLoop struct {
    provider   provider.Provider
    executors  []ToolExecutor
    maxTurns   int // safety limit on loop iterations
}

// Run executes the agentic loop:
//
//   1. Send request to provider (with tools)
//   2. Receive response
//   3. If response contains function_call items:
//      a. For each function_call:
//         - Find matching executor
//         - Execute tool (or return to client for function tools)
//         - Collect result
//      b. Append tool results to conversation
//      c. Go to step 1
//   4. If response contains only message items: return final response
//
// The loop terminates when:
//   - Model produces no tool calls (final answer)
//   - maxTurns is reached
//   - Context is cancelled
//   - An unrecoverable error occurs
func (a *AgentLoop) Run(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error
```

### Loop Flow Diagram

```
                    ┌─────────────┐
                    │  Request    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
              ┌────>│  Provider   │
              │     │  Inference  │
              │     └──────┬──────┘
              │            │
              │     ┌──────▼──────┐
              │     │ Tool calls? │
              │     └──┬──────┬───┘
              │        │      │
              │     Yes│      │No
              │        │      │
              │  ┌─────▼────┐ │  ┌────────────┐
              │  │ Execute  │ └─>│  Return    │
              │  │ Tools    │    │  Response  │
              │  └─────┬────┘    └────────────┘
              │        │
              │  ┌─────▼────────┐
              │  │ Append       │
              └──┤ Results to   │
                 │ Conversation │
                 └──────────────┘
```

## Function Calling (Client-Executed)

For function tools, antwort does not execute the function. Instead:

1. Model produces `function_call` items in the response
2. Response is returned to the client with status `requires_action` (if in agentic mode) or `completed` (if single-turn)
3. Client executes the function and submits results in a follow-up request via `input` items of type `function_call_output`

This is the standard OpenAI pattern and requires no server-side execution.

## MCP Integration

```go
// MCPExecutor connects to MCP servers and executes tools.
type MCPExecutor struct {
    clients map[string]*MCPClient // server URL -> client
}

// MCPClient wraps a connection to a single MCP server.
type MCPClient struct {
    serverURL string
    transport MCPTransport // stdio, HTTP+SSE, or streamable HTTP
}

// MCPTransport abstracts the MCP connection protocol.
type MCPTransport interface {
    Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
    Close() error
}
```

## Internal Tool Interface

```go
// InternalTool is a tool hosted and executed within antwort.
type InternalTool interface {
    ToolExecutor

    // Definition returns the tool's schema for model context.
    Definition() ToolDefinition

    // Initialize sets up the tool (e.g., build index for file search).
    Initialize(ctx context.Context) error

    // Shutdown cleans up resources.
    Shutdown(ctx context.Context) error
}

// Registry manages available internal tools.
type Registry struct {
    tools map[string]InternalTool
}

func (r *Registry) Register(tool InternalTool) error
func (r *Registry) Get(name string) (InternalTool, bool)
func (r *Registry) List() []ToolDefinition
```

## Extension Points

- **Custom internal tools**: Implement `InternalTool` interface and register with `Registry`
- **Custom MCP transports**: Implement `MCPTransport` for non-standard MCP connections
- **Provider-prefixed tools**: Use `"provider:tool_type"` naming for custom tool types
- **Loop customization**: Override `AgentLoop` behavior via options (max turns, timeout, retry policy)

## Open Questions

- Should the agentic loop support parallel tool execution (multiple function calls in one turn)?
- How to handle MCP server discovery (static config vs dynamic)?
- Should internal tools be loaded as Go plugins for extensibility without recompilation?
- How to stream intermediate tool results to the client during the agentic loop?

## Deliverables

- [ ] `pkg/tools/types.go` - ToolDefinition, ToolCall, ToolResult
- [ ] `pkg/tools/executor.go` - ToolExecutor interface
- [ ] `pkg/tools/filter.go` - ToolChoice and AllowedTools enforcement
- [ ] `pkg/tools/loop.go` - AgentLoop orchestration
- [ ] `pkg/tools/function.go` - Function calling (client-executed) handler
- [ ] `pkg/tools/mcp/client.go` - MCP client
- [ ] `pkg/tools/mcp/transport.go` - MCP transport abstraction
- [ ] `pkg/tools/internal/registry.go` - Internal tool registry
- [ ] Tests for each component
