package provider

import (
	"github.com/rhuss/antwort/pkg/api"
)

// ValidateCapabilities checks whether the given request is compatible with
// the provider's declared capabilities. Returns an APIError identifying
// the specific unsupported feature, or nil if the request is compatible.
func ValidateCapabilities(caps ProviderCapabilities, req *api.CreateResponseRequest) *api.APIError {
	// Check streaming support
	if req.Stream && !caps.Streaming {
		return api.NewInvalidRequestError("stream",
			"the configured provider does not support streaming responses")
	}

	// Check tool calling support
	if len(req.Tools) > 0 && !caps.ToolCalling {
		return api.NewInvalidRequestError("tools",
			"the configured provider does not support tool calling")
	}

	// Check for vision and audio requirements in input items
	for _, item := range req.Input {
		if item.Message == nil {
			continue
		}
		for _, part := range item.Message.Content {
			switch part.Type {
			case "input_image":
				if !caps.Vision {
					return api.NewInvalidRequestError("input",
						"the configured provider does not support image inputs")
				}
			case "input_audio":
				if !caps.Audio {
					return api.NewInvalidRequestError("input",
						"the configured provider does not support audio inputs")
				}
			case "input_video":
				// Video requires at minimum vision capability
				if !caps.Vision {
					return api.NewInvalidRequestError("input",
						"the configured provider does not support video inputs")
				}
			}
		}
	}

	return nil
}
