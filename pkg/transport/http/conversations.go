package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

func (a *Adapter) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available (no store configured)"),
			http.StatusNotImplemented,
		)
		return
	}

	var req struct {
		Name     string         `json:"name"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		transport.WriteAPIError(w, api.NewInvalidRequestError("body", "invalid request body"))
		return
	}

	now := time.Now().Unix()
	conv := &api.Conversation{
		ID:        api.NewConversationID(),
		Object:    "conversation",
		Name:      req.Name,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  req.Metadata,
	}

	if err := a.convStore.SaveConversation(r.Context(), conv); err != nil {
		transport.WriteAPIError(w, api.NewServerError(err.Error()))
		return
	}

	a.auditLogger.Log(r.Context(), "resource.created", "resource_type", "conversation", "resource_id", conv.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(conv)
}

func (a *Adapter) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available"),
			http.StatusNotImplemented,
		)
		return
	}

	id := r.PathValue("id")
	conv, err := a.convStore.GetConversation(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("conversation not found"))
		} else {
			transport.WriteAPIError(w, api.NewServerError(err.Error()))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conv)
}

func (a *Adapter) handleListConversations(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available"),
			http.StatusNotImplemented,
		)
		return
	}

	opts, apiErr := parseListOptions(r)
	if apiErr != nil {
		transport.WriteAPIError(w, apiErr)
		return
	}

	list, err := a.convStore.ListConversations(r.Context(), opts)
	if err != nil {
		transport.WriteAPIError(w, api.NewServerError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (a *Adapter) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available"),
			http.StatusNotImplemented,
		)
		return
	}

	id := r.PathValue("id")
	if err := a.convStore.DeleteConversation(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("conversation not found"))
		} else {
			transport.WriteAPIError(w, api.NewServerError(err.Error()))
		}
		return
	}

	a.auditLogger.Log(r.Context(), "resource.deleted", "resource_type", "conversation", "resource_id", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"object":  "conversation",
		"deleted": true,
	})
}

func (a *Adapter) handleAddConversationItem(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available"),
			http.StatusNotImplemented,
		)
		return
	}

	convID := r.PathValue("id")

	// Verify conversation exists.
	if _, err := a.convStore.GetConversation(r.Context(), convID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("conversation not found"))
		} else {
			transport.WriteAPIError(w, api.NewServerError(err.Error()))
		}
		return
	}

	var req struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		transport.WriteAPIError(w, api.NewInvalidRequestError("body", "invalid request body"))
		return
	}

	item := api.Item{
		ID:     api.NewItemID(),
		Type:   api.ItemTypeMessage,
		Status: api.ItemStatusCompleted,
		Message: &api.MessageData{
			Role: api.MessageRole(req.Role),
		},
	}

	if req.Role == "assistant" {
		item.Message.Output = []api.OutputContentPart{
			{Type: "output_text", Text: req.Content},
		}
	} else {
		item.Message.Content = []api.ContentPart{
			{Type: "input_text", Text: req.Content},
		}
	}

	convItem := api.ConversationItem{
		ConversationID: convID,
		Item:           item,
		CreatedAt:      time.Now().Unix(),
	}

	if err := a.convStore.AddItems(r.Context(), convID, []api.ConversationItem{convItem}); err != nil {
		transport.WriteAPIError(w, api.NewServerError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (a *Adapter) handleListConversationItems(w http.ResponseWriter, r *http.Request) {
	if a.convStore == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "conversations are not available"),
			http.StatusNotImplemented,
		)
		return
	}

	convID := r.PathValue("id")

	opts, apiErr := parseListOptions(r)
	if apiErr != nil {
		transport.WriteAPIError(w, apiErr)
		return
	}

	list, err := a.convStore.ListItems(r.Context(), convID, opts)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("conversation not found"))
		} else {
			transport.WriteAPIError(w, api.NewServerError(err.Error()))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
