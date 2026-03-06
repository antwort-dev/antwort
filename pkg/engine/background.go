package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/transport"
)

// Worker polls for queued background requests and processes them through
// the engine pipeline. It handles heartbeats, stale detection, and TTL cleanup.
type Worker struct {
	engine   *Engine
	workerID string
	cfg      config.BackgroundConfig
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// cancelRegistry tracks in-flight background request cancellation functions.
	// Used for in-process cancellation in integrated mode.
	mu             sync.RWMutex
	cancelRegistry map[string]context.CancelFunc
}

// NewWorker creates a background worker that processes queued requests.
func NewWorker(engine *Engine, cfg config.BackgroundConfig) *Worker {
	return &Worker{
		engine:         engine,
		workerID:       generateWorkerID(),
		cfg:            cfg,
		cancelRegistry: make(map[string]context.CancelFunc),
	}
}

// Start begins the worker poll loop. It blocks until the context is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	slog.Info("background worker started",
		"worker_id", w.workerID,
		"poll_interval", w.cfg.PollInterval,
	)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("background worker stopping", "worker_id", w.workerID)
			return
		case <-ticker.C:
			w.pollOnce(ctx)
		}
	}
}

// Stop initiates graceful shutdown. Waits for in-flight requests up to
// the drain timeout, then marks remaining as failed.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}

	// Wait for in-flight requests to complete.
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("background worker drained", "worker_id", w.workerID)
	case <-time.After(w.cfg.DrainTimeout):
		slog.Warn("background worker drain timeout, marking remaining as failed",
			"worker_id", w.workerID,
		)
		w.markInFlightAsFailed()
	}
}

// CancelRequest cancels an in-flight background request by response ID.
// Returns true if the request was found and cancelled.
func (w *Worker) CancelRequest(responseID string) bool {
	w.mu.RLock()
	cancelFn, ok := w.cancelRegistry[responseID]
	w.mu.RUnlock()

	if ok {
		cancelFn()
		return true
	}
	return false
}

// pollOnce runs a single poll cycle: claim work, detect stale, clean up expired.
func (w *Worker) pollOnce(ctx context.Context) {
	// Detect and mark stale requests (FR-013).
	w.detectStale(ctx)

	// Clean up expired terminal responses (FR-014).
	w.cleanupExpired(ctx)

	// Claim and process one queued request.
	resp, reqData, err := w.engine.store.ClaimQueuedResponse(ctx, w.workerID)
	if err != nil {
		slog.Error("failed to claim queued response", "error", err)
		return
	}
	if resp == nil {
		return // No work available.
	}

	slog.Info("claimed background request",
		"response_id", resp.ID,
		"worker_id", w.workerID,
	)

	w.engine.auditLogger.Log(ctx, "background.started",
		"response_id", resp.ID,
		"worker_id", w.workerID,
	)

	w.wg.Add(1)
	go w.processRequest(ctx, resp, reqData)
}

// processRequest executes a claimed background request through the engine.
func (w *Worker) processRequest(parentCtx context.Context, resp *api.Response, reqData json.RawMessage) {
	defer w.wg.Done()

	responseID := resp.ID

	// Create a cancellable context for this request.
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Register the cancel function for in-process cancellation.
	w.mu.Lock()
	w.cancelRegistry[responseID] = cancel
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.cancelRegistry, responseID)
		w.mu.Unlock()
	}()

	// Start heartbeat goroutine.
	heartbeatDone := make(chan struct{})
	go w.heartbeat(ctx, responseID, heartbeatDone)
	defer close(heartbeatDone)

	// Deserialize the original request.
	var req api.CreateResponseRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		slog.Error("failed to unmarshal background request",
			"response_id", responseID,
			"error", err,
		)
		w.markFailed(context.Background(), responseID, fmt.Errorf("invalid request data: %w", err))
		return
	}

	// Force non-background, non-streaming for worker processing.
	req.Background = false
	req.Stream = false

	// Process through the engine using a capture writer.
	cw := &captureWriter{}
	err := w.engine.CreateResponse(ctx, &req, cw)

	// Check for cancellation.
	if ctx.Err() != nil {
		w.markCancelled(context.Background(), responseID)
		return
	}

	if err != nil {
		slog.Error("background request processing failed",
			"response_id", responseID,
			"error", err,
		)
		w.markFailed(context.Background(), responseID, err)
		return
	}

	// Update the response with the result.
	w.markCompleted(context.Background(), responseID, cw)
}

// heartbeat periodically updates the worker heartbeat on the response record.
func (w *Worker) heartbeat(ctx context.Context, responseID string, done <-chan struct{}) {
	ticker := time.NewTicker(w.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			if err := w.engine.store.UpdateResponse(ctx, responseID, transport.ResponseUpdate{
				WorkerHeartbeat: &now,
			}); err != nil {
				slog.Warn("failed to update heartbeat",
					"response_id", responseID,
					"error", err,
				)
			}
		}
	}
}

// detectStale finds in_progress background responses with expired heartbeats
// and marks them as failed.
func (w *Worker) detectStale(ctx context.Context) {
	// Only memory store supports FindStaleResponses directly.
	type staleFinder interface {
		FindStaleResponses(timeout time.Duration) []string
	}

	if finder, ok := w.engine.store.(staleFinder); ok {
		staleIDs := finder.FindStaleResponses(w.cfg.StalenessTimeout)
		for _, id := range staleIDs {
			slog.Warn("marking stale background request as failed",
				"response_id", id,
				"staleness_timeout", w.cfg.StalenessTimeout,
			)
			w.markFailed(ctx, id, fmt.Errorf("worker heartbeat expired after %s", w.cfg.StalenessTimeout))
		}
	}
}

// cleanupExpired removes terminal background responses older than the TTL.
func (w *Worker) cleanupExpired(ctx context.Context) {
	if w.cfg.TTL <= 0 {
		return
	}
	cutoff := time.Now().Add(-w.cfg.TTL)
	deleted, err := w.engine.store.CleanupExpired(ctx, cutoff, w.cfg.CleanupBatchSize)
	if err != nil {
		slog.Warn("failed to cleanup expired background responses", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("cleaned up expired background responses", "count", deleted)
	}
}

// markCompleted updates a response to completed status with the worker's output.
func (w *Worker) markCompleted(ctx context.Context, responseID string, cw *captureWriter) {
	if cw.resp == nil {
		w.markFailed(ctx, responseID, fmt.Errorf("worker produced no response"))
		return
	}

	completedAt := time.Now().Unix()
	status := api.ResponseStatusCompleted
	update := transport.ResponseUpdate{
		Status:      &status,
		Output:      cw.resp.Output,
		Usage:       cw.resp.Usage,
		CompletedAt: &completedAt,
	}

	if err := w.engine.store.UpdateResponse(ctx, responseID, update); err != nil {
		slog.Error("failed to mark background request as completed",
			"response_id", responseID,
			"error", err,
		)
		return
	}
	w.engine.auditLogger.Log(ctx, "background.completed", "response_id", responseID)
}

// markFailed updates a response to failed status with error info.
func (w *Worker) markFailed(ctx context.Context, responseID string, reason error) {
	status := api.ResponseStatusFailed
	apiErr := api.NewServerError(fmt.Sprintf("background processing failed: %s", reason))
	update := transport.ResponseUpdate{
		Status: &status,
		Error:  apiErr,
	}

	if err := w.engine.store.UpdateResponse(ctx, responseID, update); err != nil {
		slog.Error("failed to mark background request as failed",
			"response_id", responseID,
			"error", err,
		)
		return
	}
	w.engine.auditLogger.LogWarn(ctx, "background.failed",
		"response_id", responseID,
		"reason", reason.Error(),
	)
}

// markCancelled updates a response to cancelled status.
func (w *Worker) markCancelled(ctx context.Context, responseID string) {
	status := api.ResponseStatusCancelled
	update := transport.ResponseUpdate{
		Status: &status,
	}

	if err := w.engine.store.UpdateResponse(ctx, responseID, update); err != nil {
		slog.Error("failed to mark background request as cancelled",
			"response_id", responseID,
			"error", err,
		)
		return
	}
	w.engine.auditLogger.Log(ctx, "background.cancelled", "response_id", responseID)
}

// markInFlightAsFailed marks all currently in-flight requests as failed
// due to shutdown timeout.
func (w *Worker) markInFlightAsFailed() {
	w.mu.RLock()
	ids := make([]string, 0, len(w.cancelRegistry))
	for id := range w.cancelRegistry {
		ids = append(ids, id)
	}
	w.mu.RUnlock()

	for _, id := range ids {
		w.markFailed(context.Background(), id, fmt.Errorf("worker shutdown timeout"))
	}
}

// captureWriter captures the response from the engine for background processing.
// It implements transport.ResponseWriter.
type captureWriter struct {
	resp *api.Response
}

func (cw *captureWriter) WriteResponse(_ context.Context, resp *api.Response) error {
	cw.resp = resp
	return nil
}

func (cw *captureWriter) WriteEvent(_ context.Context, _ api.StreamEvent) error {
	return fmt.Errorf("streaming not supported for background requests")
}

func (cw *captureWriter) Flush() error {
	return nil
}

// generateWorkerID creates a unique worker identifier.
func generateWorkerID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("worker-%s", hex.EncodeToString(b))
}
