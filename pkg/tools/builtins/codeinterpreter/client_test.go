package codeinterpreter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSandboxClient_Execute(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		req        *SandboxRequest
		wantErr    bool
		wantStatus string
		wantStdout string
	}{
		{
			name: "successful execution",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SandboxResponse{
					Status:   "success",
					Stdout:   "42\n",
					ExitCode: 0,
				})
			},
			req:        &SandboxRequest{Code: "print(42)", TimeoutSeconds: 5},
			wantStatus: "success",
			wantStdout: "42\n",
		},
		{
			name: "execution error (non-zero exit)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SandboxResponse{
					Status:   "error",
					Stderr:   "NameError: name 'x' is not defined",
					ExitCode: 1,
				})
			},
			req:        &SandboxRequest{Code: "print(x)", TimeoutSeconds: 5},
			wantStatus: "error",
		},
		{
			name: "sandbox at capacity (429)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"at capacity"}`))
			},
			req:     &SandboxRequest{Code: "print(1)", TimeoutSeconds: 5},
			wantErr: true,
		},
		{
			name: "sandbox server error (500)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal error"}`))
			},
			req:     &SandboxRequest{Code: "print(1)", TimeoutSeconds: 5},
			wantErr: true,
		},
		{
			name: "invalid JSON response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{invalid json`))
			},
			req:     &SandboxRequest{Code: "print(1)", TimeoutSeconds: 5},
			wantErr: true,
		},
		{
			name: "empty stdout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SandboxResponse{
					Status:   "success",
					Stdout:   "",
					ExitCode: 0,
				})
			},
			req:        &SandboxRequest{Code: "pass", TimeoutSeconds: 5},
			wantStatus: "success",
			wantStdout: "",
		},
		{
			name: "with files produced",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SandboxResponse{
					Status:        "success",
					Stdout:        "done\n",
					ExitCode:      0,
					FilesProduced: map[string]string{"result.csv": "YSxiCjEsMg=="},
				})
			},
			req:        &SandboxRequest{Code: "...", TimeoutSeconds: 5},
			wantStatus: "success",
			wantStdout: "done\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			client := NewSandboxClient()
			resp, err := client.Execute(context.Background(), srv.URL, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", resp.Status, tt.wantStatus)
			}

			if resp.Stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}
		})
	}
}

func TestSandboxClient_Execute_ContextTimeout(t *testing.T) {
	// Server that sleeps longer than the context deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := NewSandboxClient()
	_, err := client.Execute(ctx, srv.URL, &SandboxRequest{Code: "import time; time.sleep(10)", TimeoutSeconds: 1})

	if err == nil {
		t.Error("expected error for context timeout, got nil")
	}
}

func TestSandboxClient_Execute_Unreachable(t *testing.T) {
	client := NewSandboxClient()
	_, err := client.Execute(context.Background(), "http://localhost:1", &SandboxRequest{Code: "print(1)", TimeoutSeconds: 1})

	if err == nil {
		t.Error("expected error for unreachable server, got nil")
	}
}
