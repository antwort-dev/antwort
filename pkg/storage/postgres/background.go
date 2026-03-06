package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rhuss/antwort/pkg/api"
)

// ClaimQueuedResponse atomically transitions one queued background response
// to in_progress and assigns the given worker ID. Uses FOR UPDATE SKIP LOCKED
// to prevent duplicate processing across concurrent workers.
func (s *Store) ClaimQueuedResponse(ctx context.Context, workerID string) (*api.Response, json.RawMessage, error) {
	query := `
		UPDATE responses
		SET status = 'in_progress',
		    worker_id = $1,
		    worker_heartbeat = $2
		WHERE id = (
			SELECT id FROM responses
			WHERE status = 'queued'
			  AND background = TRUE
			  AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, status, model, previous_response_id,
		          input, output,
		          usage_input_tokens, usage_output_tokens, usage_total_tokens,
		          error, extensions, created_at, background_request
	`

	now := time.Now()

	var resp api.Response
	var status string
	var prevID *string
	var inputJSON, outputJSON []byte
	var errorJSON, extensionsJSON *[]byte
	var backgroundReq *[]byte
	var usageIn, usageOut, usageTotal int

	err := s.pool.QueryRow(ctx, query, workerID, now).Scan(
		&resp.ID, &status, &resp.Model, &prevID,
		&inputJSON, &outputJSON,
		&usageIn, &usageOut, &usageTotal,
		&errorJSON, &extensionsJSON, &resp.CreatedAt, &backgroundReq,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil // no queued responses
	}
	if err != nil {
		return nil, nil, fmt.Errorf("claiming queued response: %w", err)
	}

	resp.Object = "response"
	resp.Status = api.ResponseStatus(status)
	resp.PreviousResponseID = prevID
	resp.Background = true

	if err := json.Unmarshal(inputJSON, &resp.Input); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling input: %w", err)
	}
	if err := json.Unmarshal(outputJSON, &resp.Output); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling output: %w", err)
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
			return nil, nil, fmt.Errorf("unmarshaling extensions: %w", err)
		}
	}

	var reqData json.RawMessage
	if backgroundReq != nil {
		reqData = *backgroundReq
	}

	return &resp, reqData, nil
}

// CleanupExpired deletes terminal background responses older than the given cutoff.
func (s *Store) CleanupExpired(ctx context.Context, olderThan time.Time, batchSize int) (int, error) {
	query := `
		DELETE FROM responses
		WHERE id IN (
			SELECT id FROM responses
			WHERE background = TRUE
			  AND status IN ('completed', 'failed', 'cancelled')
			  AND created_at < $1
			LIMIT $2
		)
	`

	result, err := s.pool.Exec(ctx, query, olderThan.Unix(), batchSize)
	if err != nil {
		return 0, fmt.Errorf("cleaning up expired responses: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// FindStaleResponses returns IDs of background responses that have been
// in_progress longer than the staleness timeout without a heartbeat update.
func (s *Store) FindStaleResponses(stalenessTimeout time.Duration) []string {
	ctx := context.Background()
	cutoff := time.Now().Add(-stalenessTimeout)

	query := `
		SELECT id FROM responses
		WHERE status = 'in_progress'
		  AND background = TRUE
		  AND deleted_at IS NULL
		  AND worker_heartbeat < $1
	`

	rows, err := s.pool.Query(ctx, query, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// SaveBackgroundRequest stores the serialized request alongside the response.
func (s *Store) SaveBackgroundRequest(ctx context.Context, id string, reqData json.RawMessage) error {
	query := `UPDATE responses SET background_request = $1, background = TRUE WHERE id = $2`
	_, err := s.pool.Exec(ctx, query, reqData, id)
	if err != nil {
		return fmt.Errorf("saving background request: %w", err)
	}
	return nil
}
