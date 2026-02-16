package api

import (
	"fmt"
	"strings"
)

// ValidationConfig holds configurable limits for request validation.
type ValidationConfig struct {
	MaxInputItems  int
	MaxContentSize int
	MaxTools       int
}

// DefaultValidationConfig returns a ValidationConfig with sensible defaults.
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxInputItems:  1000,
		MaxContentSize: 10 * 1024 * 1024, // 10MB
		MaxTools:       128,
	}
}

// ValidateRequest checks a CreateResponseRequest for validity. It returns an
// *APIError describing the first validation failure, or nil if the request is valid.
func ValidateRequest(req *CreateResponseRequest, cfg ValidationConfig) *APIError {
	if req.Model == "" {
		return NewInvalidRequestError("model", "model is required")
	}

	if len(req.Input) == 0 {
		return NewInvalidRequestError("input", "input must contain at least one item")
	}

	if cfg.MaxInputItems > 0 && len(req.Input) > cfg.MaxInputItems {
		return NewInvalidRequestError("input",
			fmt.Sprintf("input exceeds maximum of %d items", cfg.MaxInputItems))
	}

	if cfg.MaxTools > 0 && len(req.Tools) > cfg.MaxTools {
		return NewInvalidRequestError("tools",
			fmt.Sprintf("tools exceeds maximum of %d", cfg.MaxTools))
	}

	if req.MaxOutputTokens != nil && *req.MaxOutputTokens <= 0 {
		return NewInvalidRequestError("max_output_tokens", "max_output_tokens must be positive")
	}

	if req.Temperature != nil {
		if *req.Temperature < 0.0 || *req.Temperature > 2.0 {
			return NewInvalidRequestError("temperature", "temperature must be between 0.0 and 2.0")
		}
	}

	if req.TopP != nil {
		if *req.TopP < 0.0 || *req.TopP > 1.0 {
			return NewInvalidRequestError("top_p", "top_p must be between 0.0 and 1.0")
		}
	}

	if req.Truncation != "" && req.Truncation != "auto" && req.Truncation != "disabled" {
		return NewInvalidRequestError("truncation", "truncation must be 'auto' or 'disabled'")
	}

	// Validate tool_choice references an existing tool when forcing a specific function.
	if req.ToolChoice != nil && req.ToolChoice.Function != nil {
		name := req.ToolChoice.Function.Name
		found := false
		for _, tool := range req.Tools {
			if tool.Name == name {
				found = true
				break
			}
		}
		if !found {
			return NewInvalidRequestError("tool_choice",
				fmt.Sprintf("tool_choice references unknown tool %q", name))
		}
	}

	return nil
}

// ValidateItem checks an Item for structural validity.
func ValidateItem(item *Item) *APIError {
	if item.ID != "" && !ValidateItemID(item.ID) {
		return NewInvalidRequestError("id", "invalid item ID format")
	}

	if item.Type == "" {
		return NewInvalidRequestError("type", "item type is required")
	}

	// Check for standard types or extension types.
	if !isStandardItemType(item.Type) && !IsExtensionType(item.Type) {
		return NewInvalidRequestError("type",
			fmt.Sprintf("invalid item type %q: must be a standard type or use provider:type format", item.Type))
	}

	// For extension types, extension data must be present.
	if IsExtensionType(item.Type) {
		if item.Extension == nil {
			return NewInvalidRequestError("extension", "extension items must have extension data")
		}
		return nil
	}

	// For standard types, exactly one type-specific field must be populated.
	count := 0
	if item.Message != nil {
		count++
	}
	if item.FunctionCall != nil {
		count++
	}
	if item.FunctionCallOutput != nil {
		count++
	}
	if item.Reasoning != nil {
		count++
	}

	if count != 1 {
		return NewInvalidRequestError("type",
			"exactly one type-specific field must be populated")
	}

	// Verify the populated field matches the type.
	switch item.Type {
	case ItemTypeMessage:
		if item.Message == nil {
			return NewInvalidRequestError("message", "message field required for message type")
		}
	case ItemTypeFunctionCall:
		if item.FunctionCall == nil {
			return NewInvalidRequestError("function_call", "function_call field required for function_call type")
		}
	case ItemTypeFunctionCallOutput:
		if item.FunctionCallOutput == nil {
			return NewInvalidRequestError("function_call_output", "function_call_output field required for function_call_output type")
		}
	case ItemTypeReasoning:
		if item.Reasoning == nil {
			return NewInvalidRequestError("reasoning", "reasoning field required for reasoning type")
		}
	}

	return nil
}

// IsStateless returns true if the request is configured for stateless mode
// (store explicitly set to false).
func IsStateless(req *CreateResponseRequest) bool {
	return req.Store != nil && !*req.Store
}

func isStandardItemType(t ItemType) bool {
	switch t {
	case ItemTypeMessage, ItemTypeFunctionCall, ItemTypeFunctionCallOutput, ItemTypeReasoning:
		return true
	}
	return false
}

// ValidateStatelessConstraints checks stateless-specific constraints.
// This should be called after ValidateRequest for requests with store=false.
func ValidateStatelessConstraints(req *CreateResponseRequest) *APIError {
	if IsStateless(req) && req.PreviousResponseID != "" {
		return NewInvalidRequestError("previous_response_id",
			"previous_response_id cannot be used with store=false")
	}
	return nil
}

// storeDefault is used internally; exported for testing.
func storeDefault() bool {
	return true
}

// ResolveStore returns the effective store value, defaulting to true when nil.
func ResolveStore(req *CreateResponseRequest) bool {
	if req.Store != nil {
		return *req.Store
	}
	return true
}

// ValidateExtensionType checks whether the given type string is a valid extension
// type (matches "provider:type" pattern with non-empty segments).
func ValidateExtensionType(t string) bool {
	parts := strings.SplitN(t, ":", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
