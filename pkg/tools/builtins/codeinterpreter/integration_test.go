package codeinterpreter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// These tests use the real sandbox-server binary started as a subprocess.
// They require Python to be installed (available on GitHub Actions Ubuntu runners).
// Skipped when running with -short flag.

func TestIntegration_CodeInterpreter_ExecuteSimple(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sandboxURL := startSandboxServer(t)

	provider, err := New(map[string]any{
		"sandbox_url":       sandboxURL,
		"execution_timeout": float64(30),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := tools.ToolCall{
		ID:        "call_test_001",
		Name:      "code_interpreter",
		Arguments: `{"code": "print(6 * 7)"}`,
	}

	result, err := provider.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Output)
	}

	var data api.CodeInterpreterCallData
	if err := json.Unmarshal([]byte(result.Output), &data); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, result.Output)
	}

	if data.Code != "print(6 * 7)" {
		t.Errorf("code = %q, want %q", data.Code, "print(6 * 7)")
	}

	foundLogs := false
	for _, out := range data.Outputs {
		if out.Type == "logs" && strings.Contains(out.Logs, "42") {
			foundLogs = true
		}
	}
	if !foundLogs {
		t.Errorf("expected logs containing '42', got outputs: %+v", data.Outputs)
	}
}

func TestIntegration_CodeInterpreter_ExecuteWithRequirements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("skipping: uv not installed (required for pip package installation)")
	}

	sandboxURL := startSandboxServer(t)

	provider, err := New(map[string]any{
		"sandbox_url":       sandboxURL,
		"execution_timeout": float64(60),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Use a lightweight package that's quick to install.
	call := tools.ToolCall{
		ID:   "call_test_002",
		Name: "code_interpreter",
		Arguments: `{
			"code": "import six; print(six.text_type.__name__)",
			"requirements": ["six"]
		}`,
	}

	result, err := provider.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Output)
	}

	var data api.CodeInterpreterCallData
	if err := json.Unmarshal([]byte(result.Output), &data); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	foundLogs := false
	for _, out := range data.Outputs {
		if out.Type == "logs" && strings.Contains(out.Logs, "str") {
			foundLogs = true
		}
	}
	if !foundLogs {
		t.Errorf("expected logs containing 'str', got outputs: %+v", data.Outputs)
	}
}

func TestIntegration_CodeInterpreter_ExecuteError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sandboxURL := startSandboxServer(t)

	provider, err := New(map[string]any{
		"sandbox_url":       sandboxURL,
		"execution_timeout": float64(10),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := tools.ToolCall{
		ID:        "call_test_003",
		Name:      "code_interpreter",
		Arguments: `{"code": "raise ValueError('test error')"}`,
	}

	result, err := provider.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Python errors are still returned as successful tool results (not provider errors).
	// The stderr should contain the error message.
	if result.IsError {
		t.Fatalf("expected tool result (not provider error), got: %s", result.Output)
	}

	var data api.CodeInterpreterCallData
	if err := json.Unmarshal([]byte(result.Output), &data); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	foundError := false
	for _, out := range data.Outputs {
		if out.Type == "logs" && strings.Contains(out.Logs, "ValueError") {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected logs containing 'ValueError', got outputs: %+v", data.Outputs)
	}
}

func TestIntegration_CodeInterpreter_InvalidArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sandboxURL := startSandboxServer(t)

	provider, err := New(map[string]any{
		"sandbox_url":       sandboxURL,
		"execution_timeout": float64(10),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	call := tools.ToolCall{
		ID:        "call_test_004",
		Name:      "code_interpreter",
		Arguments: `{"not_code": "print(1)"}`,
	}

	result, err := provider.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for missing code field")
	}
}

// startSandboxServer builds and starts the real sandbox-server binary as a subprocess.
// Returns the base URL (http://localhost:<port>).
// The server is killed when the test completes.
func startSandboxServer(t *testing.T) string {
	t.Helper()

	// Check Python is available.
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping integration test")
	}

	// Build the sandbox-server binary.
	tmpDir := t.TempDir()
	binaryPath := tmpDir + "/sandbox-server"

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/sandbox-server")
	build.Dir = findRepoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building sandbox-server: %v\n%s", err, out)
	}

	// Find a free port.
	port := freePort(t)

	// Start the server.
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SANDBOX_PORT=%d", port),
		"SANDBOX_MODE=python",
		"SANDBOX_MAX_CONCURRENT=2",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting sandbox-server: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	// Wait for the server to be ready.
	url := fmt.Sprintf("http://localhost:%d", port)
	waitForReady(t, url+"/health", 10*time.Second)

	return url
}

// findRepoRoot walks up from the current directory to find the repo root (where go.mod is).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

// freePort returns an available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("finding free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// waitForReady polls the health endpoint until the server responds or the timeout expires.
func waitForReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("sandbox-server did not become ready at %s within %s", url, timeout)
}
