// Command sandbox-server runs an HTTP server inside agent-sandbox pods
// that executes code in isolated subprocesses.
//
// Configuration:
//
//	SANDBOX_PORT         - Listen port (default: 8080)
//	SANDBOX_MODE         - Runtime mode: python, golang, node, shell (default: auto-detect)
//	SANDBOX_MAX_CONCURRENT - Max concurrent executions (default: 3)
//	SANDBOX_PYTHON_INDEX - Python package index URL (default: https://pypi.org/simple/)
//	SANDBOX_OUTPUT_DIR   - Output directory name within temp dir (default: output)
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	port := envOr("SANDBOX_PORT", "8080")
	mode := envOr("SANDBOX_MODE", "")
	maxConcurrent := envOrInt("SANDBOX_MAX_CONCURRENT", 3)
	pythonIndex := envOr("SANDBOX_PYTHON_INDEX", "https://pypi.org/simple/")
	outputDirName := envOr("SANDBOX_OUTPUT_DIR", "output")

	// Resolve mode: explicit or auto-detect.
	if mode == "" {
		detected := detectMode()
		if detected == "" {
			slog.Error("no supported runtime found in PATH (tried: python3, go, node, bash)")
			os.Exit(1)
		}
		mode = detected
	} else {
		// Validate explicit mode.
		if err := validateMode(mode); err != nil {
			slog.Error("invalid mode", "mode", mode, "error", err.Error())
			os.Exit(1)
		}
	}

	// Detect runtime version.
	runtimeVersion := detectRuntimeVersion(mode)

	srv := &sandboxServer{
		mode:           mode,
		runtimeVersion: runtimeVersion,
		maxConcurrent:  int32(maxConcurrent),
		pythonIndex:    pythonIndex,
		outputDirName:  outputDirName,
		startTime:      time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /execute", srv.handleExecute)
	mux.HandleFunc("GET /health", srv.handleHealth)

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Long timeout for code execution.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("sandbox server starting", "port", port, "mode", mode, "runtime", runtimeVersion, "max_concurrent", maxConcurrent)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutdownCtx)
}

// --- Server ---

type sandboxServer struct {
	mode           string // python, golang, node, shell
	runtimeVersion string // e.g., "Python 3.12.12", "go1.25", "v22.0.0"
	maxConcurrent  int32
	currentLoad    atomic.Int32
	pythonIndex    string
	outputDirName  string
	startTime      time.Time
}

// modeConfig returns the interpreter command, file extension, and extra
// environment variables for the active mode.
func (s *sandboxServer) modeConfig(tmpDir, outputDir string) (cmd []string, ext string, env []string) {
	env = []string{"OUTPUT_DIR=" + outputDir}

	switch s.mode {
	case "python":
		pyLibs := filepath.Join(tmpDir, ".pylibs")
		return []string{"python3"}, ".py", append(env, "PYTHONPATH="+pyLibs)
	case "golang":
		return []string{"go", "run"}, ".go", env
	case "node":
		return []string{"node"}, ".js", env
	case "shell":
		return []string{"bash"}, ".sh", env
	default:
		return []string{"python3"}, ".py", env
	}
}

// --- Execute handler ---

type executeRequest struct {
	Code           string            `json:"code"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Requirements   []string          `json:"requirements,omitempty"`
	Files          map[string]string `json:"files,omitempty"`
}

type executeResponse struct {
	Status         string            `json:"status"`
	Stdout         string            `json:"stdout"`
	Stderr         string            `json:"stderr"`
	ExitCode       int               `json:"exit_code"`
	ExecutionTimeMs int64            `json:"execution_time_ms"`
	FilesProduced  map[string]string `json:"files_produced,omitempty"`
}

func (s *sandboxServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	// Check capacity.
	current := s.currentLoad.Add(1)
	defer s.currentLoad.Add(-1)

	if current > s.maxConcurrent {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("at capacity (%d/%d concurrent executions)", current, s.maxConcurrent),
		})
		return
	}

	// Parse request.
	var req executeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10*1024*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Truncate code for logging (first 120 chars).
	codePreview := req.Code
	if len(codePreview) > 120 {
		codePreview = codePreview[:120] + "..."
	}
	slog.Info("execute request",
		"code", codePreview,
		"timeout", req.TimeoutSeconds,
		"requirements", len(req.Requirements),
		"files", len(req.Files),
	)

	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 30 // Default timeout.
	}

	// Create temporary working directory.
	tmpDir, err := os.MkdirTemp("", "sandbox-exec-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp dir: "+err.Error())
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create output directory.
	outputDir := filepath.Join(tmpDir, s.outputDirName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create output dir: "+err.Error())
		return
	}

	// Write input files.
	for name, b64Content := range req.Files {
		content, err := base64.StdEncoding.DecodeString(b64Content)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to decode file %q: %v", name, err))
			return
		}
		filePath := filepath.Join(tmpDir, filepath.Base(name)) // Prevent path traversal.
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to write file %q: %v", name, err))
			return
		}
	}

	// Install requirements if specified and mode supports it.
	if len(req.Requirements) > 0 {
		installErr := s.installRequirements(r.Context(), tmpDir, req.Requirements, req.TimeoutSeconds)
		if installErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(executeResponse{
				Status:   "error",
				Stderr:   "package installation failed: " + installErr.Error(),
				ExitCode: -1,
			})
			return
		}
	}

	// Get mode-specific interpreter, file extension, and env.
	interpreter, fileExt, extraEnv := s.modeConfig(tmpDir, outputDir)

	// Write the code to a file.
	codePath := filepath.Join(tmpDir, "script"+fileExt)
	if err := os.WriteFile(codePath, []byte(req.Code), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write code: "+err.Error())
		return
	}

	// Execute with timeout.
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(req.TimeoutSeconds)*time.Second)
	defer cancel()

	startTime := time.Now()
	cmdArgs := append(interpreter[1:], codePath)
	cmd := exec.CommandContext(ctx, interpreter[0], cmdArgs...)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), extraEnv...)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	execErr := cmd.Run()
	duration := time.Since(startTime)

	// Determine exit code.
	exitCode := 0
	status := "success"
	if execErr != nil {
		status = "error"
		// Check timeout first (context deadline takes precedence over exit error).
		if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
			if stderrBuf.Len() == 0 {
				stderrBuf.WriteString(fmt.Sprintf("execution timed out after %d seconds", req.TimeoutSeconds))
			}
		} else if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Collect output files.
	filesProduced := collectOutputFiles(outputDir)

	// Log completion.
	stdoutPreview := stdoutBuf.String()
	if len(stdoutPreview) > 200 {
		stdoutPreview = stdoutPreview[:200] + "..."
	}
	fileCount := len(filesProduced)
	slog.Info("execute complete",
		"status", status,
		"exit_code", exitCode,
		"duration_ms", duration.Milliseconds(),
		"stdout_len", stdoutBuf.Len(),
		"stdout", stdoutPreview,
		"files_produced", fileCount,
	)

	// Return response.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executeResponse{
		Status:          status,
		Stdout:          stdoutBuf.String(),
		Stderr:          stderrBuf.String(),
		ExitCode:        exitCode,
		ExecutionTimeMs: duration.Milliseconds(),
		FilesProduced:   filesProduced,
	})
}

// installRequirements installs packages based on the active mode.
// Python: uv pip install. Node: npm install. Go/shell: skip.
func (s *sandboxServer) installRequirements(ctx context.Context, workDir string, requirements []string, timeoutSecs int) error {
	switch s.mode {
	case "python":
		return s.installPythonRequirements(ctx, workDir, requirements, timeoutSecs)
	case "node":
		return s.installNodeRequirements(ctx, workDir, requirements, timeoutSecs)
	default:
		// Go and shell: silently skip.
		return nil
	}
}

func (s *sandboxServer) installPythonRequirements(ctx context.Context, workDir string, requirements []string, timeoutSecs int) error {
	installCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	targetDir := filepath.Join(workDir, ".pylibs")
	args := []string{"pip", "install", "--system", "--target", targetDir, "--index-url", s.pythonIndex}
	args = append(args, requirements...)

	cmd := exec.CommandContext(installCtx, "uv", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(output))
	}
	return nil
}

func (s *sandboxServer) installNodeRequirements(ctx context.Context, workDir string, requirements []string, timeoutSecs int) error {
	installCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	args := []string{"install"}
	args = append(args, requirements...)

	cmd := exec.CommandContext(installCtx, "npm", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(output))
	}
	return nil
}

// collectOutputFiles reads files from the output directory and encodes them as base64.
func collectOutputFiles(outputDir string) map[string]string {
	entries, err := os.ReadDir(outputDir)
	if err != nil || len(entries) == 0 {
		return nil
	}

	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(outputDir, entry.Name()))
		if err != nil {
			continue
		}
		files[entry.Name()] = base64.StdEncoding.EncodeToString(content)
	}

	if len(files) == 0 {
		return nil
	}
	return files
}

// --- Health handler ---

type healthResponse struct {
	Status         string `json:"status"`
	Mode           string `json:"mode"`
	RuntimeVersion string `json:"runtime_version"`
	Capacity       int    `json:"capacity"`
	CurrentLoad    int    `json:"current_load"`
	UptimeSecs     int64  `json:"uptime_seconds"`
}

func (s *sandboxServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		Status:         "healthy",
		Mode:           s.mode,
		RuntimeVersion: s.runtimeVersion,
		Capacity:       int(s.maxConcurrent),
		CurrentLoad:    int(s.currentLoad.Load()),
		UptimeSecs:     int64(time.Since(s.startTime).Seconds()),
	})
}

// --- Mode detection ---

// detectMode checks for runtimes in PATH in priority order.
func detectMode() string {
	checks := []struct {
		mode string
		cmd  string
	}{
		{"python", "python3"},
		{"golang", "go"},
		{"node", "node"},
		{"shell", "bash"},
	}
	for _, c := range checks {
		if _, err := exec.LookPath(c.cmd); err == nil {
			return c.mode
		}
	}
	return ""
}

// validateMode checks that the configured mode is valid and the runtime is available.
func validateMode(mode string) error {
	cmdMap := map[string]string{
		"python": "python3",
		"golang": "go",
		"node":   "node",
		"shell":  "bash",
	}

	cmd, ok := cmdMap[mode]
	if !ok {
		return fmt.Errorf("unsupported mode %q (supported: python, golang, node, shell)", mode)
	}

	if _, err := exec.LookPath(cmd); err != nil {
		return fmt.Errorf("mode=%s but %q not found in PATH", mode, cmd)
	}

	return nil
}

// detectRuntimeVersion returns the version string for the active runtime.
func detectRuntimeVersion(mode string) string {
	var cmd *exec.Cmd
	switch mode {
	case "python":
		cmd = exec.Command("python3", "--version")
	case "golang":
		cmd = exec.Command("go", "version")
	case "node":
		cmd = exec.Command("node", "--version")
	case "shell":
		cmd = exec.Command("bash", "--version")
	default:
		return "unknown"
	}

	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Return first line, trimmed.
	version := strings.TrimSpace(string(output))
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}
	return version
}

// --- Helpers ---

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}
