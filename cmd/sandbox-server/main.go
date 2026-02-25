// Command sandbox-server runs an HTTP server inside agent-sandbox pods
// that executes Python code in isolated subprocesses.
//
// Configuration:
//
//	SANDBOX_PORT         - Listen port (default: 8080)
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
	maxConcurrent := envOrInt("SANDBOX_MAX_CONCURRENT", 3)
	pythonIndex := envOr("SANDBOX_PYTHON_INDEX", "https://pypi.org/simple/")
	outputDirName := envOr("SANDBOX_OUTPUT_DIR", "output")

	srv := &sandboxServer{
		maxConcurrent: int32(maxConcurrent),
		pythonIndex:   pythonIndex,
		outputDirName: outputDirName,
		startTime:     time.Now(),
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
		slog.Info("sandbox server starting", "port", port, "max_concurrent", maxConcurrent)
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
	maxConcurrent int32
	currentLoad   atomic.Int32
	pythonIndex   string
	outputDirName string
	startTime     time.Time
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

	// Install requirements if specified.
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

	// Write the code to a file.
	codePath := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(codePath, []byte(req.Code), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write code: "+err.Error())
		return
	}

	// Execute with timeout.
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(req.TimeoutSeconds)*time.Second)
	defer cancel()

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "python3", codePath)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "OUTPUT_DIR="+outputDir)

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

// installRequirements runs uv pip install for the specified packages.
func (s *sandboxServer) installRequirements(ctx context.Context, workDir string, requirements []string, timeoutSecs int) error {
	installCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	args := []string{"pip", "install", "--index-url", s.pythonIndex}
	args = append(args, requirements...)

	cmd := exec.CommandContext(installCtx, "uv", args...)
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
	Status      string `json:"status"`
	Capacity    int    `json:"capacity"`
	CurrentLoad int    `json:"current_load"`
	UptimeSecs  int64  `json:"uptime_seconds"`
}

func (s *sandboxServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		Status:      "healthy",
		Capacity:    int(s.maxConcurrent),
		CurrentLoad: int(s.currentLoad.Load()),
		UptimeSecs:  int64(time.Since(s.startTime).Seconds()),
	})
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
