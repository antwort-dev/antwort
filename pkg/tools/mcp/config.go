package mcp

// Config holds the configuration for all MCP server connections.
type Config struct {
	// Servers is the list of MCP server configurations to connect to.
	Servers []ServerConfig
}

// ServerConfig describes a single MCP server connection.
type ServerConfig struct {
	// Name is the logical name for this server, used for logging and
	// identification when routing tool calls.
	Name string `json:"name"`

	// Transport is the transport type to use: "sse" or "streamable-http".
	// If empty, defaults to "streamable-http".
	Transport string `json:"transport"`

	// URL is the MCP server endpoint URL.
	URL string `json:"url"`

	// Headers contains additional HTTP headers to send with requests,
	// typically used for authentication (API keys, bearer tokens, etc.).
	Headers map[string]string `json:"headers,omitempty"`
}
