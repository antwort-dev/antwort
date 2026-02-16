package api

import "fmt"

// ErrorType represents the category of an API error.
type ErrorType string

const (
	ErrorTypeServerError      ErrorType = "server_error"
	ErrorTypeInvalidRequest   ErrorType = "invalid_request"
	ErrorTypeNotFound         ErrorType = "not_found"
	ErrorTypeModelError       ErrorType = "model_error"
	ErrorTypeTooManyRequests  ErrorType = "too_many_requests"
)

// APIError represents a structured API error with type, code, param, and message.
type APIError struct {
	Type    ErrorType `json:"type"`
	Code    string    `json:"code,omitempty"`
	Param   string    `json:"param,omitempty"`
	Message string    `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Param != "" {
		return fmt.Sprintf("%s: %s (param: %s)", e.Type, e.Message, e.Param)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ErrorResponse wraps an APIError for JSON serialization as the top-level error response.
type ErrorResponse struct {
	Error *APIError `json:"error"`
}

// NewInvalidRequestError creates an APIError for invalid request parameters.
func NewInvalidRequestError(param, message string) *APIError {
	return &APIError{
		Type:    ErrorTypeInvalidRequest,
		Param:   param,
		Message: message,
	}
}

// NewNotFoundError creates an APIError for resources that cannot be found.
func NewNotFoundError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}

// NewServerError creates an APIError for internal server errors.
func NewServerError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeServerError,
		Message: message,
	}
}

// NewModelError creates an APIError for model-related errors.
func NewModelError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeModelError,
		Message: message,
	}
}

// NewTooManyRequestsError creates an APIError for rate limiting.
func NewTooManyRequestsError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeTooManyRequests,
		Message: message,
	}
}
