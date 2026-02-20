package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// MCPClient wraps an MCP SDK Client and ClientSession for a single
// MCP server connection. It handles connection lifecycle, tool discovery,
// and tool execution.
type MCPClient struct {
	cfg     ServerConfig
	client  *mcp.Client
	session *mcp.ClientSession

	mu            sync.Mutex
	cachedTools   []api.ToolDefinition
	toolsResolved bool
}

// NewMCPClient creates a new MCPClient for the given server configuration.
// Call Connect to establish the connection.
func NewMCPClient(cfg ServerConfig) *MCPClient {
	return &MCPClient{cfg: cfg}
}

// Connect establishes the MCP connection to the server, performing the
// protocol handshake. For testing, an optional transport can be provided
// to bypass URL-based transport creation.
func (c *MCPClient) Connect(ctx context.Context) error {
	return c.ConnectWithTransport(ctx, nil)
}

// ConnectWithTransport establishes the MCP connection using the given
// transport. If transport is nil, a transport is created from the
// server configuration.
func (c *MCPClient) ConnectWithTransport(ctx context.Context, transport mcp.Transport) error {
	c.client = mcp.NewClient(
		&mcp.Implementation{
			Name:    "antwort",
			Version: "1.0.0",
		},
		&mcp.ClientOptions{
			Capabilities: &mcp.ClientCapabilities{},
		},
	)

	if transport == nil {
		t, err := c.createTransport()
		if err != nil {
			return fmt.Errorf("creating transport for %q: %w", c.cfg.Name, err)
		}
		transport = t
	}

	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connecting to MCP server %q: %w", c.cfg.Name, err)
	}
	c.session = session
	return nil
}

// createTransport creates an MCP transport based on the server configuration.
func (c *MCPClient) createTransport() (mcp.Transport, error) {
	httpClient := c.buildHTTPClient()

	switch c.cfg.Transport {
	case "sse":
		transport := &mcp.SSEClientTransport{
			Endpoint: c.cfg.URL,
		}
		if httpClient != nil {
			transport.HTTPClient = httpClient
		}
		return transport, nil

	case "streamable-http", "":
		transport := &mcp.StreamableClientTransport{
			Endpoint: c.cfg.URL,
		}
		if httpClient != nil {
			transport.HTTPClient = httpClient
		}
		return transport, nil

	default:
		return nil, fmt.Errorf("unsupported transport type %q", c.cfg.Transport)
	}
}

// buildHTTPClient returns an HTTP client with the appropriate transport
// for authentication. Returns nil if no auth or custom headers are configured.
func (c *MCPClient) buildHTTPClient() *http.Client {
	var authProvider AuthProvider

	switch c.cfg.Auth.Type {
	case "oauth_client_credentials":
		authProvider = NewOAuthClientCredentials(
			c.cfg.Auth.TokenURL,
			c.cfg.Auth.ClientID,
			c.cfg.Auth.ClientSecret,
			c.cfg.Auth.Scopes,
		)
	}

	// Build transport chain: static headers + auth provider.
	hasStaticHeaders := len(c.cfg.Headers) > 0
	if !hasStaticHeaders && authProvider == nil {
		return nil
	}

	return &http.Client{
		Transport: &authAwareTransport{
			base:         http.DefaultTransport,
			headers:      c.cfg.Headers,
			authProvider: authProvider,
		},
	}
}

// authAwareTransport is an http.RoundTripper that adds static headers and
// dynamically obtained auth headers to every request.
type authAwareTransport struct {
	base         http.RoundTripper
	headers      map[string]string
	authProvider AuthProvider
}

func (t *authAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Apply static headers first.
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Apply auth provider headers (may override static headers, e.g. Authorization).
	if t.authProvider != nil {
		authHeaders, err := t.authProvider.GetHeaders(req.Context())
		if err != nil {
			return nil, fmt.Errorf("getting auth headers: %w", err)
		}
		for k, v := range authHeaders {
			req.Header.Set(k, v)
		}
	}

	return t.base.RoundTrip(req)
}

// headerTransport is an http.RoundTripper that adds custom headers to
// every request. Kept for backward compatibility.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// DiscoverTools queries the MCP server for available tools, converts them
// to api.ToolDefinition format, and caches the results. Subsequent calls
// return the cached tools unless the cache is invalidated.
func (c *MCPClient) DiscoverTools(ctx context.Context) ([]api.ToolDefinition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.toolsResolved {
		return c.cachedTools, nil
	}

	if c.session == nil {
		return nil, fmt.Errorf("MCP client %q not connected", c.cfg.Name)
	}

	var toolDefs []api.ToolDefinition
	for tool, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("listing tools from %q: %w", c.cfg.Name, err)
		}
		td, convErr := convertTool(tool)
		if convErr != nil {
			return nil, fmt.Errorf("converting tool %q from %q: %w", tool.Name, c.cfg.Name, convErr)
		}
		toolDefs = append(toolDefs, td)
	}

	c.cachedTools = toolDefs
	c.toolsResolved = true
	return toolDefs, nil
}

// CallTool executes a tool call on the MCP server and returns the result.
func (c *MCPClient) CallTool(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	if c.session == nil {
		return nil, fmt.Errorf("MCP client %q not connected", c.cfg.Name)
	}

	// Parse the arguments from JSON string to a generic map.
	var args map[string]any
	if call.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return &tools.ToolResult{
				CallID:  call.ID,
				Output:  fmt.Sprintf("invalid arguments JSON: %v", err),
				IsError: true,
			}, nil
		}
	}

	params := &mcp.CallToolParams{
		Name:      call.Name,
		Arguments: args,
	}

	result, err := c.session.CallTool(ctx, params)
	if err != nil {
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("MCP tool call error: %v", err),
			IsError: true,
		}, nil
	}

	return convertResult(call.ID, result), nil
}

// Close closes the MCP session.
func (c *MCPClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

// convertTool converts an MCP Tool to an api.ToolDefinition.
func convertTool(t *mcp.Tool) (api.ToolDefinition, error) {
	var params json.RawMessage
	if t.InputSchema != nil {
		data, err := json.Marshal(t.InputSchema)
		if err != nil {
			return api.ToolDefinition{}, fmt.Errorf("marshaling input schema: %w", err)
		}
		params = data
	}

	return api.ToolDefinition{
		Type:        "function",
		Name:        t.Name,
		Description: t.Description,
		Parameters:  params,
		Strict:      false,
	}, nil
}

// convertResult converts an MCP CallToolResult to a tools.ToolResult.
func convertResult(callID string, result *mcp.CallToolResult) *tools.ToolResult {
	// Extract text content from the result.
	var output string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			if output != "" {
				output += "\n"
			}
			output += tc.Text
		}
	}

	return &tools.ToolResult{
		CallID:  callID,
		Output:  output,
		IsError: result.IsError,
	}
}
