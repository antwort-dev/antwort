// Command mcp-test-server runs a simple MCP server for testing the
// antwort MCP client integration. Provides "get_time" and "echo" tools.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "antwort-test-mcp", Version: "v1.0.0"},
		nil,
	)

	// Add "get_time" tool.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_time",
		Description: "Returns the current UTC time",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct{}, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Current time: %s", time.Now().UTC().Format(time.RFC3339))},
			},
		}, struct{}{}, nil
	})

	// Add "echo" tool.
	type EchoInput struct {
		Message string `json:"message" jsonschema_description:"The message to echo back"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echoes the provided message back",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input EchoInput) (*mcp.CallToolResult, struct{}, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Echo: %s", input.Message)},
			},
		}, struct{}{}, nil
	})

	// Serve via streamable HTTP on /mcp.
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)

	httpMux := http.NewServeMux()
	httpMux.Handle("/mcp", handler)
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})

	log.Printf("MCP test server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, httpMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
