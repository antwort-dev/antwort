package codeinterpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/registry"
)

// Ensure CodeInterpreterProvider implements FunctionProvider.
var _ registry.FunctionProvider = (*CodeInterpreterProvider)(nil)

// Config holds configuration for the code interpreter provider.
type Config struct {
	// SandboxURL is the static URL of a sandbox server (development mode).
	// Mutually exclusive with SandboxTemplate.
	SandboxURL string

	// SandboxTemplate is the name of the SandboxTemplate CRD for SandboxClaim mode.
	// Mutually exclusive with SandboxURL.
	SandboxTemplate string

	// SandboxNamespace is the Kubernetes namespace for SandboxClaims.
	SandboxNamespace string

	// ExecutionTimeout is the default code execution timeout in seconds.
	ExecutionTimeout int

	// ClaimTimeout is how long to wait for a SandboxClaim to be bound (seconds).
	ClaimTimeout int
}

// SandboxAcquirer abstracts sandbox acquisition. Implementations exist for
// static URL mode (returns a fixed URL) and SandboxClaim mode (creates CRDs).
type SandboxAcquirer interface {
	// Acquire returns a sandbox URL to use for execution.
	// The release function must be called after execution to clean up.
	Acquire(ctx context.Context) (sandboxURL string, release func(), err error)
}

// CodeInterpreterProvider is a FunctionProvider that executes Python code
// in sandbox pods via the sandbox server REST API.
type CodeInterpreterProvider struct {
	acquirer SandboxAcquirer
	client   *SandboxClient
	config   Config
}

// New creates a new CodeInterpreterProvider from configuration settings.
func New(settings map[string]any) (*CodeInterpreterProvider, error) {
	cfg := Config{
		ExecutionTimeout: 60,
		ClaimTimeout:     30,
	}

	if v, ok := settings["sandbox_url"].(string); ok && v != "" {
		cfg.SandboxURL = v
	}
	if v, ok := settings["sandbox_template"].(string); ok && v != "" {
		cfg.SandboxTemplate = v
	}
	if v, ok := settings["sandbox_namespace"].(string); ok && v != "" {
		cfg.SandboxNamespace = v
	}
	if v, ok := settings["execution_timeout"].(float64); ok && v > 0 {
		cfg.ExecutionTimeout = int(v)
	}
	if v, ok := settings["claim_timeout"].(float64); ok && v > 0 {
		cfg.ClaimTimeout = int(v)
	}

	// Validate mutual exclusion.
	if cfg.SandboxURL != "" && cfg.SandboxTemplate != "" {
		return nil, fmt.Errorf("code_interpreter: sandbox_url and sandbox_template are mutually exclusive")
	}
	if cfg.SandboxURL == "" && cfg.SandboxTemplate == "" {
		return nil, fmt.Errorf("code_interpreter: either sandbox_url or sandbox_template must be set")
	}

	var acquirer SandboxAcquirer
	if cfg.SandboxURL != "" {
		acquirer = &staticURLAcquirer{url: cfg.SandboxURL}
	} else {
		// SandboxClaim mode requires the Kubernetes adapter (future).
		return nil, fmt.Errorf("code_interpreter: sandbox_template mode requires the Kubernetes adapter (not yet implemented, use sandbox_url for now)")
	}

	return &CodeInterpreterProvider{
		acquirer: acquirer,
		client:   NewSandboxClient(),
		config:   cfg,
	}, nil
}

// Name returns the provider name.
func (p *CodeInterpreterProvider) Name() string {
	return "code_interpreter"
}

// Tools returns the tool definitions for this provider.
func (p *CodeInterpreterProvider) Tools() []api.ToolDefinition {
	params, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "Python code to execute",
			},
			"requirements": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Python packages to install before execution (e.g., ['pandas', 'numpy'])",
			},
		},
		"required": []string{"code"},
	})

	return []api.ToolDefinition{
		{
			Type:        "function",
			Name:        "code_interpreter",
			Description: "Execute Python code in an isolated sandbox. Use this to analyze data, perform calculations, or process files.",
			Parameters:  params,
		},
	}
}

// Execute runs the code_interpreter tool.
func (p *CodeInterpreterProvider) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	// Parse arguments.
	var args struct {
		Code         string   `json:"code"`
		Requirements []string `json:"requirements"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}, nil
	}

	if args.Code == "" {
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  "code is required",
			IsError: true,
		}, nil
	}

	// Acquire a sandbox.
	sandboxURL, release, err := p.acquirer.Acquire(ctx)
	if err != nil {
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("failed to acquire sandbox: %v", err),
			IsError: true,
		}, nil
	}
	defer release()

	// Execute in sandbox.
	resp, err := p.client.Execute(ctx, sandboxURL, &SandboxRequest{
		Code:           args.Code,
		TimeoutSeconds: p.config.ExecutionTimeout,
		Requirements:   args.Requirements,
	})
	if err != nil {
		slog.Warn("code_interpreter execution failed",
			"call_id", call.ID,
			"error", err.Error(),
		)
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("sandbox execution failed: %v", err),
			IsError: true,
		}, nil
	}

	// Format as code_interpreter_call output.
	output := formatCodeInterpreterOutput(args.Code, resp)

	return &tools.ToolResult{
		CallID: call.ID,
		Output: output,
	}, nil
}

// CanExecute returns true for the code_interpreter tool.
func (p *CodeInterpreterProvider) CanExecute(toolName string) bool {
	return toolName == "code_interpreter"
}

// Routes returns nil (no HTTP management routes for code_interpreter).
func (p *CodeInterpreterProvider) Routes() []registry.Route {
	return nil
}

// Collectors returns nil (no custom Prometheus collectors).
func (p *CodeInterpreterProvider) Collectors() []prometheus.Collector {
	return nil
}

// Close releases resources.
func (p *CodeInterpreterProvider) Close() error {
	return nil
}

// formatCodeInterpreterOutput creates a JSON string matching the
// code_interpreter_call output format.
func formatCodeInterpreterOutput(code string, resp *SandboxResponse) string {
	outputs := []api.CodeInterpreterOutput{}

	// Add logs (stdout + stderr).
	logText := resp.Stdout
	if resp.Stderr != "" {
		if logText != "" {
			logText += "\n"
		}
		logText += resp.Stderr
	}
	if logText != "" {
		outputs = append(outputs, api.CodeInterpreterOutput{
			Type: "logs",
			Logs: logText,
		})
	}

	// Add file outputs.
	for name := range resp.FilesProduced {
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".svg" || ext == ".gif" {
			outputs = append(outputs, api.CodeInterpreterOutput{
				Type: "image",
				Image: &api.CodeInterpreterOutputImage{
					FileID: name, // Use filename as file_id for now.
				},
			})
		} else {
			// Non-image files: include as logs with filename prefix.
			outputs = append(outputs, api.CodeInterpreterOutput{
				Type: "logs",
				Logs: fmt.Sprintf("[file: %s]", name),
			})
		}
	}

	data := api.CodeInterpreterCallData{
		Code:    code,
		Outputs: outputs,
	}

	result, _ := json.Marshal(data)
	return string(result)
}

// staticURLAcquirer returns a fixed sandbox URL (development mode).
type staticURLAcquirer struct {
	url string
}

func (a *staticURLAcquirer) Acquire(_ context.Context) (string, func(), error) {
	return a.url, func() {}, nil // No cleanup needed for static URL.
}
