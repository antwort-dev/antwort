package transport

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rhuss/antwort/pkg/api"
)

// ResponseCreator handles the core create-response operation.
// It is the primary handler contract, available in both stateless and
// stateful deployments. The implementation receives a request and writes
// the result (streaming events or a complete response) to the ResponseWriter.
type ResponseCreator interface {
	CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error
}

// ResponseCreatorFunc is an adapter that allows using an ordinary function
// as a ResponseCreator.
type ResponseCreatorFunc func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error

// CreateResponse calls f(ctx, req, w).
func (f ResponseCreatorFunc) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
	return f(ctx, req, w)
}

// ListOptions controls pagination, filtering, and ordering for list operations.
type ListOptions struct {
	After      string // Cursor: return items after this ID.
	Before     string // Cursor: return items before this ID.
	Limit      int    // Maximum number of items to return (default 20, max 100).
	Model      string // Filter responses by model name (list responses only).
	Order      string // Sort order: "asc" or "desc" (default "desc").
	Status     string // Filter responses by status (e.g., "queued", "in_progress").
	Background *bool  // Filter responses by background flag.
}

// ResponseList holds a paginated list of responses.
type ResponseList struct {
	Object  string          `json:"object"`
	Data    []*api.Response `json:"data"`
	HasMore bool            `json:"has_more"`
	FirstID string          `json:"first_id"`
	LastID  string          `json:"last_id"`
}

// ItemList holds a paginated list of input items.
type ItemList struct {
	Object  string     `json:"object"`
	Data    []api.Item `json:"data"`
	HasMore bool       `json:"has_more"`
	FirstID string     `json:"first_id"`
	LastID  string     `json:"last_id"`
}

// ResponseStore handles persistence, retrieval, and deletion of stored responses.
// It is only available in stateful deployments with persistence configured.
type ResponseStore interface {
	// SaveResponse persists a completed response to the store.
	SaveResponse(ctx context.Context, resp *api.Response) error

	// GetResponse retrieves a response by ID. Returns an error if the
	// response does not exist or has been deleted (soft delete).
	GetResponse(ctx context.Context, id string) (*api.Response, error)

	// GetResponseForChain retrieves a response by ID for chain reconstruction.
	// Unlike GetResponse, this includes soft-deleted responses so that
	// conversation chains remain intact when intermediate responses are deleted.
	GetResponseForChain(ctx context.Context, id string) (*api.Response, error)

	// DeleteResponse soft-deletes a response by ID.
	DeleteResponse(ctx context.Context, id string) error

	// ListResponses returns a paginated list of stored responses.
	// Results are filtered by tenant (when present in context) and
	// optionally by model. Supports cursor-based pagination and ordering.
	ListResponses(ctx context.Context, opts ListOptions) (*ResponseList, error)

	// GetInputItems returns a paginated list of input items for a response.
	// Returns storage.ErrNotFound if the response does not exist.
	GetInputItems(ctx context.Context, responseID string, opts ListOptions) (*ItemList, error)

	// HealthCheck verifies the store connection is functional.
	HealthCheck(ctx context.Context) error

	// UpdateResponse updates specific fields on an existing response.
	// Only non-nil fields in the update are applied.
	UpdateResponse(ctx context.Context, id string, update ResponseUpdate) error

	// ClaimQueuedResponse atomically transitions one queued response to
	// in_progress and assigns the given worker ID. Returns the response
	// and the original serialized request. Returns nil, nil, nil if no
	// queued responses are available.
	ClaimQueuedResponse(ctx context.Context, workerID string) (*api.Response, json.RawMessage, error)

	// CleanupExpired deletes terminal background responses older than the
	// given cutoff time. Returns the number of responses deleted.
	CleanupExpired(ctx context.Context, olderThan time.Time, batchSize int) (int, error)

	// Close releases database connections and resources.
	Close() error
}

// ResponseUpdate holds the fields to update on an existing response.
// Only non-nil/non-zero fields are applied.
type ResponseUpdate struct {
	Status          *api.ResponseStatus
	Output          []api.Item
	Error           *api.APIError
	Usage           *api.Usage
	CompletedAt     *int64
	WorkerHeartbeat *time.Time
}

// ConversationList is a paginated list of conversations.
type ConversationList struct {
	Object  string              `json:"object"`
	Data    []*api.Conversation `json:"data"`
	HasMore bool                `json:"has_more"`
	FirstID string              `json:"first_id"`
	LastID  string              `json:"last_id"`
}

// ConversationStore handles persistence and retrieval of conversations.
// It is only available in stateful deployments with persistence configured.
type ConversationStore interface {
	// SaveConversation creates or updates a conversation.
	SaveConversation(ctx context.Context, conv *api.Conversation) error

	// GetConversation retrieves a conversation by ID.
	GetConversation(ctx context.Context, id string) (*api.Conversation, error)

	// DeleteConversation soft-deletes a conversation by ID.
	DeleteConversation(ctx context.Context, id string) error

	// ListConversations returns a paginated list of conversations for the user.
	ListConversations(ctx context.Context, opts ListOptions) (*ConversationList, error)

	// AddItems appends items to a conversation.
	AddItems(ctx context.Context, conversationID string, items []api.ConversationItem) error

	// ListItems returns a paginated list of items in a conversation.
	ListItems(ctx context.Context, conversationID string, opts ListOptions) (*ItemList, error)
}

// ResponseWriter abstracts streaming and non-streaming output for the handler.
// The transport layer creates a ResponseWriter for each request and provides
// it to the handler. The handler uses WriteEvent for streaming responses or
// WriteResponse for non-streaming responses.
//
// WriteEvent and WriteResponse are mutually exclusive on a single writer
// instance. Calling WriteEvent after WriteResponse (or vice versa) returns
// an error. Calling WriteEvent after a terminal event (response.completed,
// response.failed, or response.cancelled) also returns an error.
type ResponseWriter interface {
	// WriteEvent sends a single streaming event. Returns an error if called
	// after a terminal event has been sent or after WriteResponse was called.
	WriteEvent(ctx context.Context, event api.StreamEvent) error

	// WriteResponse sends a complete non-streaming response. Returns an error
	// if called after WriteEvent was called on this writer.
	WriteResponse(ctx context.Context, resp *api.Response) error

	// Flush ensures buffered data is sent to the client. Returns an error
	// if the client has disconnected.
	Flush() error
}
