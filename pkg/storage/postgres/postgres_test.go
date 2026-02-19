package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
)

func init() {
	// Configure testcontainers to use podman.
	// Detect the podman socket from `podman machine inspect`.
	if os.Getenv("DOCKER_HOST") == "" {
		out, err := exec.Command("podman", "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}").Output()
		if err == nil {
			sock := strings.TrimSpace(string(out))
			if sock != "" {
				os.Setenv("DOCKER_HOST", "unix://"+sock)
			}
		}
	}
	// Ryuk needs privileged mode with podman.
	if os.Getenv("TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED") == "" {
		os.Setenv("TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED", "true")
	}
}

// setupTestDB starts a PostgreSQL container and returns a connected Store.
// Tests are skipped if Docker is not available.
func setupTestDB(t *testing.T) *Store {
	t.Helper()

	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("SKIP_INTEGRATION=true, skipping PostgreSQL integration tests")
	}

	// Verify podman is running.
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not found, skipping integration tests")
	}

	ctx := context.Background()

	container, err := pgmodule.Run(ctx,
		"postgres:16-alpine",
		pgmodule.WithDatabase("antwort_test"),
		pgmodule.WithUsername("test"),
		pgmodule.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Skipf("skipping: could not start PostgreSQL container (is podman running?): %v", err)
	}

	t.Cleanup(func() {
		container.Terminate(context.Background())
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	store, err := New(ctx, Config{
		DSN:            connStr,
		MaxConns:       5,
		MinConns:       1,
		MigrateOnStart: true,
	})
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func makeTestResponse(id string) *api.Response {
	return &api.Response{
		ID:     id,
		Object: "response",
		Status: api.ResponseStatusCompleted,
		Model:  "test-model",
		Input: []api.Item{
			{ID: "item_in1", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
				Message: &api.MessageData{Role: api.RoleUser,
					Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}}},
		},
		Output: []api.Item{
			{ID: "item_out1", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
				Message: &api.MessageData{Role: api.RoleAssistant,
					Output: []api.OutputContentPart{{Type: "output_text", Text: "hi there"}}}},
		},
		Usage:     &api.Usage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8},
		CreatedAt: time.Now().Unix(),
	}
}

func TestPostgres_SaveAndGet(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	resp := makeTestResponse("resp_pg_test1_" + fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := store.SaveResponse(ctx, resp); err != nil {
		t.Fatalf("SaveResponse failed: %v", err)
	}

	got, err := store.GetResponse(ctx, resp.ID)
	if err != nil {
		t.Fatalf("GetResponse failed: %v", err)
	}

	if got.ID != resp.ID {
		t.Errorf("ID = %q, want %q", got.ID, resp.ID)
	}
	if got.Model != "test-model" {
		t.Errorf("Model = %q, want %q", got.Model, "test-model")
	}
	if got.Status != api.ResponseStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, api.ResponseStatusCompleted)
	}
	if len(got.Input) != 1 {
		t.Errorf("len(Input) = %d, want 1", len(got.Input))
	}
	if len(got.Output) != 1 {
		t.Errorf("len(Output) = %d, want 1", len(got.Output))
	}
	if got.Usage == nil || got.Usage.InputTokens != 5 {
		t.Errorf("Usage.InputTokens = %v, want 5", got.Usage)
	}
}

func TestPostgres_GetNotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := store.GetResponse(ctx, "resp_nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgres_SoftDelete(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	resp := makeTestResponse("resp_pg_del_" + fmt.Sprintf("%d", time.Now().UnixNano()))
	store.SaveResponse(ctx, resp)

	if err := store.DeleteResponse(ctx, resp.ID); err != nil {
		t.Fatalf("DeleteResponse failed: %v", err)
	}

	// GetResponse should return not-found.
	_, err := store.GetResponse(ctx, resp.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// GetResponseForChain should still return it.
	got, err := store.GetResponseForChain(ctx, resp.ID)
	if err != nil {
		t.Fatalf("GetResponseForChain should return deleted response: %v", err)
	}
	if got.ID != resp.ID {
		t.Errorf("chain ID = %q, want %q", got.ID, resp.ID)
	}
}

func TestPostgres_DuplicateSave(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	resp := makeTestResponse("resp_pg_dup_" + fmt.Sprintf("%d", time.Now().UnixNano()))
	store.SaveResponse(ctx, resp)

	err := store.SaveResponse(ctx, resp)
	if !errors.Is(err, storage.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestPostgres_HealthCheck(t *testing.T) {
	store := setupTestDB(t)
	if err := store.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestPostgres_ChainReconstruction(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	ts := fmt.Sprintf("%d", time.Now().UnixNano())

	respA := makeTestResponse("resp_chain_a_" + ts)
	respB := makeTestResponse("resp_chain_b_" + ts)
	respB.PreviousResponseID = &respA.ID
	respC := makeTestResponse("resp_chain_c_" + ts)
	respC.PreviousResponseID = &respB.ID

	store.SaveResponse(ctx, respA)
	store.SaveResponse(ctx, respB)
	store.SaveResponse(ctx, respC)

	// Delete middle response.
	store.DeleteResponse(ctx, respB.ID)

	// Chain reconstruction should still work.
	gotB, err := store.GetResponseForChain(ctx, respB.ID)
	if err != nil {
		t.Fatalf("GetResponseForChain(B) failed: %v", err)
	}
	if gotB.PreviousResponseID == nil || *gotB.PreviousResponseID != respA.ID {
		t.Errorf("chain link: B.previous = %v, want %q", gotB.PreviousResponseID, respA.ID)
	}
}

func TestPostgres_TenantIsolation(t *testing.T) {
	store := setupTestDB(t)

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	ctxA := storage.SetTenant(context.Background(), "tenant-a")
	ctxB := storage.SetTenant(context.Background(), "tenant-b")

	resp := makeTestResponse("resp_tenant_" + ts)
	store.SaveResponse(ctxA, resp)

	// Tenant A can retrieve.
	if _, err := store.GetResponse(ctxA, resp.ID); err != nil {
		t.Fatalf("tenant A should see own response: %v", err)
	}

	// Tenant B cannot retrieve.
	if _, err := store.GetResponse(ctxB, resp.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Error("tenant B should not see tenant A's response")
	}

	// No tenant can retrieve (single-tenant mode).
	if _, err := store.GetResponse(context.Background(), resp.ID); err != nil {
		t.Fatalf("no-tenant should see all: %v", err)
	}
}
