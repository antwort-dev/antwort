package memory

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rhuss/antwort/pkg/api"
)

// SaveBackgroundRequest stores the serialized request alongside the response
// for later reconstruction by workers.
func (s *Store) SaveBackgroundRequest(ctx context.Context, id string, reqData json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return nil // silently ignore if response doesn't exist
	}
	e.backgroundReq = reqData
	return nil
}

// ClaimQueuedResponse atomically transitions one queued response to in_progress
// and assigns the given worker ID. Returns the response and the original serialized
// request. Returns nil, nil, nil if no queued responses are available.
func (s *Store) ClaimQueuedResponse(ctx context.Context, workerID string) (*api.Response, json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the oldest queued background response.
	var oldest *entry
	var oldestTime int64

	for _, e := range s.entries {
		if e.deletedAt != nil {
			continue
		}
		if e.resp.Status != api.ResponseStatusQueued || !e.resp.Background {
			continue
		}
		if oldest == nil || e.resp.CreatedAt < oldestTime {
			oldest = e
			oldestTime = e.resp.CreatedAt
		}
	}

	if oldest == nil {
		return nil, nil, nil
	}

	// Atomically claim: transition to in_progress and assign worker.
	oldest.resp.Status = api.ResponseStatusInProgress
	oldest.workerID = workerID
	now := time.Now()
	oldest.workerHeartbeat = &now

	return oldest.resp, oldest.backgroundReq, nil
}

// CleanupExpired deletes terminal background responses older than the given cutoff.
// Returns the number of responses deleted.
func (s *Store) CleanupExpired(ctx context.Context, olderThan time.Time, batchSize int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := olderThan.Unix()
	deleted := 0

	for id, e := range s.entries {
		if deleted >= batchSize {
			break
		}
		if !e.resp.Background {
			continue
		}
		// Only clean up terminal statuses.
		switch e.resp.Status {
		case api.ResponseStatusCompleted, api.ResponseStatusFailed, api.ResponseStatusCancelled:
		default:
			continue
		}
		if e.resp.CreatedAt < cutoff {
			if e.lruElem != nil {
				s.lruList.Remove(e.lruElem)
			}
			delete(s.entries, id)
			deleted++
		}
	}

	return deleted, nil
}

// FindStaleResponses returns IDs of background responses that have been
// in_progress longer than the staleness timeout without a heartbeat update.
func (s *Store) FindStaleResponses(stalenessTimeout time.Duration) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-stalenessTimeout)
	var stale []string

	for _, e := range s.entries {
		if e.deletedAt != nil {
			continue
		}
		if e.resp.Status != api.ResponseStatusInProgress || !e.resp.Background {
			continue
		}
		if e.workerHeartbeat != nil && e.workerHeartbeat.Before(cutoff) {
			stale = append(stale, e.resp.ID)
		}
	}

	return stale
}
