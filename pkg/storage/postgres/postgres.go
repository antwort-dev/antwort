// Package postgres provides a PostgreSQL implementation of transport.ResponseStore.
// It uses pgx/v5 for connection pooling and JSONB for structured item storage.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// Store is a PostgreSQL-backed ResponseStore.
type Store struct {
	pool *pgxpool.Pool
}

// Ensure Store implements transport.ResponseStore at compile time.
var _ transport.ResponseStore = (*Store)(nil)

// New creates a new PostgreSQL store with the given configuration.
// If MigrateOnStart is true, schema migrations are applied automatically.
func New(ctx context.Context, cfg Config) (*Store, error) {
	cfg.defaults()

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parsing DSN: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	s := &Store{pool: pool}

	if cfg.MigrateOnStart {
		if err := s.migrate(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("running migrations: %w", err)
		}
	}

	return s, nil
}

// SaveResponse persists a completed response.
func (s *Store) SaveResponse(ctx context.Context, resp *api.Response) error {
	start := time.Now()
	tenantID := storage.GetTenant(ctx)
	owner := storage.GetOwner(ctx)

	inputJSON, err := json.Marshal(resp.Input)
	if err != nil {
		return fmt.Errorf("marshaling input: %w", err)
	}

	outputJSON, err := json.Marshal(resp.Output)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}

	var errorJSON []byte
	if resp.Error != nil {
		errorJSON, err = json.Marshal(resp.Error)
		if err != nil {
			return fmt.Errorf("marshaling error: %w", err)
		}
	}

	var extensionsJSON []byte
	if resp.Extensions != nil {
		extensionsJSON, err = json.Marshal(resp.Extensions)
		if err != nil {
			return fmt.Errorf("marshaling extensions: %w", err)
		}
	}

	var usageIn, usageOut, usageTotal int
	if resp.Usage != nil {
		usageIn = resp.Usage.InputTokens
		usageOut = resp.Usage.OutputTokens
		usageTotal = resp.Usage.TotalTokens
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO responses (
			id, tenant_id, owner, status, model, previous_response_id,
			input, output,
			usage_input_tokens, usage_output_tokens, usage_total_tokens,
			error, extensions, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		resp.ID, tenantID, owner, string(resp.Status), resp.Model, resp.PreviousResponseID,
		inputJSON, outputJSON,
		usageIn, usageOut, usageTotal,
		nullJSON(errorJSON), nullJSON(extensionsJSON), resp.CreatedAt,
	)

	if err != nil {
		observability.StorageOperationsTotal.WithLabelValues("postgres", "save", "error").Inc()
		observability.StorageOperationDuration.WithLabelValues("postgres", "save").Observe(time.Since(start).Seconds())
		s.recordConnectionsActive()
		if isDuplicateKey(err) {
			return storage.ErrConflict
		}
		return fmt.Errorf("inserting response: %w", err)
	}

	observability.StorageOperationsTotal.WithLabelValues("postgres", "save", "success").Inc()
	observability.StorageOperationDuration.WithLabelValues("postgres", "save").Observe(time.Since(start).Seconds())
	s.recordConnectionsActive()
	return nil
}

// recordConnectionsActive updates the active connections gauge from the pool stats.
func (s *Store) recordConnectionsActive() {
	stat := s.pool.Stat()
	observability.StorageConnectionsActive.Set(float64(stat.AcquiredConns()))
}

// GetResponse retrieves a response by ID, excluding soft-deleted responses.
func (s *Store) GetResponse(ctx context.Context, id string) (*api.Response, error) {
	return s.getResponse(ctx, id, true)
}

// GetResponseForChain retrieves a response by ID for chain reconstruction,
// including soft-deleted responses.
func (s *Store) GetResponseForChain(ctx context.Context, id string) (*api.Response, error) {
	return s.getResponse(ctx, id, false)
}

// getResponse is the internal retrieval implementation.
func (s *Store) getResponse(ctx context.Context, id string, excludeDeleted bool) (*api.Response, error) {
	start := time.Now()
	tenantID := storage.GetTenant(ctx)
	owner := storage.GetOwner(ctx)
	isAdminCaller := storage.GetAdmin(ctx)

	query := `
		SELECT id, status, model, previous_response_id,
		       input, output,
		       usage_input_tokens, usage_output_tokens, usage_total_tokens,
		       error, extensions, created_at
		FROM responses
		WHERE id = $1
	`
	args := []any{id}
	argIdx := 2

	if excludeDeleted {
		query += " AND deleted_at IS NULL"
	}

	if tenantID != "" {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	// Owner filtering: skip if no identity, admin, or empty owner (legacy).
	if owner != "" && !isAdminCaller {
		query += fmt.Sprintf(" AND (owner = '' OR owner = $%d)", argIdx)
		args = append(args, owner)
	}

	var resp api.Response
	var status string
	var prevID *string
	var inputJSON, outputJSON []byte
	var errorJSON, extensionsJSON *[]byte
	var usageIn, usageOut, usageTotal int

	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&resp.ID, &status, &resp.Model, &prevID,
		&inputJSON, &outputJSON,
		&usageIn, &usageOut, &usageTotal,
		&errorJSON, &extensionsJSON, &resp.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		observability.StorageOperationsTotal.WithLabelValues("postgres", "get", "error").Inc()
		observability.StorageOperationDuration.WithLabelValues("postgres", "get").Observe(time.Since(start).Seconds())
		s.recordConnectionsActive()
		return nil, storage.ErrNotFound
	}
	if err != nil {
		observability.StorageOperationsTotal.WithLabelValues("postgres", "get", "error").Inc()
		observability.StorageOperationDuration.WithLabelValues("postgres", "get").Observe(time.Since(start).Seconds())
		s.recordConnectionsActive()
		return nil, fmt.Errorf("querying response: %w", err)
	}

	observability.StorageOperationsTotal.WithLabelValues("postgres", "get", "success").Inc()
	observability.StorageOperationDuration.WithLabelValues("postgres", "get").Observe(time.Since(start).Seconds())
	s.recordConnectionsActive()

	resp.Object = "response"
	resp.Status = api.ResponseStatus(status)
	resp.PreviousResponseID = prevID

	if err := json.Unmarshal(inputJSON, &resp.Input); err != nil {
		return nil, fmt.Errorf("unmarshaling input: %w", err)
	}
	if err := json.Unmarshal(outputJSON, &resp.Output); err != nil {
		return nil, fmt.Errorf("unmarshaling output: %w", err)
	}

	resp.Usage = &api.Usage{
		InputTokens:  usageIn,
		OutputTokens: usageOut,
		TotalTokens:  usageTotal,
	}

	if errorJSON != nil {
		var apiErr api.APIError
		if err := json.Unmarshal(*errorJSON, &apiErr); err == nil {
			resp.Error = &apiErr
		}
	}

	if extensionsJSON != nil {
		if err := json.Unmarshal(*extensionsJSON, &resp.Extensions); err != nil {
			return nil, fmt.Errorf("unmarshaling extensions: %w", err)
		}
	}

	return &resp, nil
}

// DeleteResponse soft-deletes a response by setting deleted_at.
func (s *Store) DeleteResponse(ctx context.Context, id string) error {
	start := time.Now()
	tenantID := storage.GetTenant(ctx)
	owner := storage.GetOwner(ctx)
	isAdminCaller := storage.GetAdmin(ctx)

	query := "UPDATE responses SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL"
	args := []any{time.Now(), id}
	argIdx := 3

	if tenantID != "" {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if owner != "" && !isAdminCaller {
		query += fmt.Sprintf(" AND (owner = '' OR owner = $%d)", argIdx)
		args = append(args, owner)
	}

	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		observability.StorageOperationsTotal.WithLabelValues("postgres", "delete", "error").Inc()
		observability.StorageOperationDuration.WithLabelValues("postgres", "delete").Observe(time.Since(start).Seconds())
		s.recordConnectionsActive()
		return fmt.Errorf("deleting response: %w", err)
	}

	if result.RowsAffected() == 0 {
		observability.StorageOperationsTotal.WithLabelValues("postgres", "delete", "error").Inc()
		observability.StorageOperationDuration.WithLabelValues("postgres", "delete").Observe(time.Since(start).Seconds())
		s.recordConnectionsActive()
		return storage.ErrNotFound
	}

	observability.StorageOperationsTotal.WithLabelValues("postgres", "delete", "success").Inc()
	observability.StorageOperationDuration.WithLabelValues("postgres", "delete").Observe(time.Since(start).Seconds())
	s.recordConnectionsActive()
	return nil
}

// HealthCheck verifies the database connection.
func (s *Store) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close releases the connection pool.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// UpdateResponse updates specific fields on an existing response.
func (s *Store) UpdateResponse(ctx context.Context, id string, update transport.ResponseUpdate) error {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if update.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*update.Status))
		argIdx++
	}
	if update.Output != nil {
		outputJSON, err := json.Marshal(update.Output)
		if err != nil {
			return fmt.Errorf("marshaling output: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("output = $%d", argIdx))
		args = append(args, outputJSON)
		argIdx++
	}
	if update.Error != nil {
		errorJSON, err := json.Marshal(update.Error)
		if err != nil {
			return fmt.Errorf("marshaling error: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("error = $%d", argIdx))
		args = append(args, errorJSON)
		argIdx++
	}
	if update.Usage != nil {
		setClauses = append(setClauses,
			fmt.Sprintf("usage_input_tokens = $%d", argIdx),
			fmt.Sprintf("usage_output_tokens = $%d", argIdx+1),
			fmt.Sprintf("usage_total_tokens = $%d", argIdx+2),
		)
		args = append(args, update.Usage.InputTokens, update.Usage.OutputTokens, update.Usage.TotalTokens)
		argIdx += 3
	}
	if update.CompletedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("completed_at = $%d", argIdx))
		args = append(args, *update.CompletedAt)
		argIdx++
	}
	if update.WorkerHeartbeat != nil {
		setClauses = append(setClauses, fmt.Sprintf("worker_heartbeat = $%d", argIdx))
		args = append(args, *update.WorkerHeartbeat)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	query := "UPDATE responses SET "
	for i, c := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += c
	}
	query += fmt.Sprintf(" WHERE id = $%d AND deleted_at IS NULL", argIdx)
	args = append(args, id)

	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating response: %w", err)
	}
	if result.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// nullString converts an empty string to nil for nullable TEXT columns.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullJSON converts nil/empty byte slices to nil for nullable JSONB columns.
func nullJSON(b []byte) *[]byte {
	if len(b) == 0 {
		return nil
	}
	return &b
}

// ListResponses returns a paginated list of stored responses.
// TODO(028): Implement SQL-based listing with cursor pagination.
func (s *Store) ListResponses(ctx context.Context, opts transport.ListOptions) (*transport.ResponseList, error) {
	return nil, fmt.Errorf("ListResponses not yet implemented for PostgreSQL")
}

// GetInputItems returns input items for a stored response.
// TODO(028): Implement SQL-based input item retrieval with pagination.
func (s *Store) GetInputItems(ctx context.Context, responseID string, opts transport.ListOptions) (*transport.ItemList, error) {
	return nil, fmt.Errorf("GetInputItems not yet implemented for PostgreSQL")
}

// isDuplicateKey checks if the error is a PostgreSQL unique violation (23505).
func isDuplicateKey(err error) bool {
	return err != nil && contains(err.Error(), "23505")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
