// Package codeinterpreter provides a FunctionProvider that executes Python
// code in isolated sandbox pods via the sandbox server REST API.
package codeinterpreter

// SandboxRequest is the request body for POST /execute on the sandbox server.
type SandboxRequest struct {
	Code           string            `json:"code"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Requirements   []string          `json:"requirements,omitempty"`
	Files          map[string]string `json:"files,omitempty"`
}

// SandboxResponse is the response from POST /execute on the sandbox server.
type SandboxResponse struct {
	Status          string            `json:"status"`
	Stdout          string            `json:"stdout"`
	Stderr          string            `json:"stderr"`
	ExitCode        int               `json:"exit_code"`
	ExecutionTimeMs int64             `json:"execution_time_ms"`
	FilesProduced   map[string]string `json:"files_produced,omitempty"`
}
