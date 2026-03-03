package http

import (
	"encoding/json"
	"net/http"

	"github.com/rhuss/antwort/pkg/agent"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/transport"
)

func (a *Adapter) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if a.profileResolver == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "agent profiles are not configured"),
			http.StatusNotImplemented,
		)
		return
	}

	lister, ok := a.profileResolver.(*agent.ConfigResolver)
	if !ok {
		transport.WriteAPIError(w, api.NewServerError("profile resolver does not support listing"))
		return
	}

	summaries := lister.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   summaries,
	})
}
