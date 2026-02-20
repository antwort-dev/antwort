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

	// Auth configures the authentication method for this MCP server.
	// If set, the auth provider is used instead of static Headers for
	// authentication. Static Headers are still sent alongside auth headers.
	Auth MCPAuthConfig `json:"auth,omitempty"`
}

// MCPAuthConfig describes the authentication configuration for an MCP server.
type MCPAuthConfig struct {
	// Type is the authentication method: "static" or "oauth_client_credentials".
	Type string `json:"type" yaml:"type"`

	// TokenURL is the OAuth 2.0 token endpoint URL (for oauth_client_credentials).
	TokenURL string `json:"token_url,omitempty" yaml:"token_url"`

	// ClientID is the OAuth 2.0 client identifier.
	ClientID string `json:"client_id,omitempty" yaml:"client_id"`

	// ClientIDFile is the path to a file containing the client ID.
	ClientIDFile string `json:"client_id_file,omitempty" yaml:"client_id_file"`

	// ClientSecret is the OAuth 2.0 client secret.
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret"`

	// ClientSecretFile is the path to a file containing the client secret.
	ClientSecretFile string `json:"client_secret_file,omitempty" yaml:"client_secret_file"`

	// Scopes is the list of OAuth 2.0 scopes to request.
	Scopes []string `json:"scopes,omitempty" yaml:"scopes"`
}
