package api

import (
	"encoding/json"
	"testing"
)

func TestAPIErrorInterface(t *testing.T) {
	var _ error = &APIError{}
}

func TestAPIErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			"with param",
			&APIError{Type: ErrorTypeInvalidRequest, Param: "model", Message: "is required"},
			"invalid_request: is required (param: model)",
		},
		{
			"without param",
			&APIError{Type: ErrorTypeServerError, Message: "internal failure"},
			"server_error: internal failure",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name      string
		err       *APIError
		wantType  ErrorType
		wantParam string
	}{
		{"invalid request", NewInvalidRequestError("model", "is required"), ErrorTypeInvalidRequest, "model"},
		{"not found", NewNotFoundError("response not found"), ErrorTypeNotFound, ""},
		{"server error", NewServerError("internal failure"), ErrorTypeServerError, ""},
		{"model error", NewModelError("model overloaded"), ErrorTypeModelError, ""},
		{"too many requests", NewTooManyRequestsError("rate limit exceeded"), ErrorTypeTooManyRequests, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", tt.err.Type, tt.wantType)
			}
			if tt.err.Param != tt.wantParam {
				t.Errorf("Param = %q, want %q", tt.err.Param, tt.wantParam)
			}
		})
	}
}

func TestAPIErrorJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
	}{
		{"invalid request", NewInvalidRequestError("model", "is required")},
		{"not found", NewNotFoundError("not found")},
		{"server error", NewServerError("internal")},
		{"model error", NewModelError("overloaded")},
		{"too many requests", NewTooManyRequestsError("rate limit")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var got APIError
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got.Type != tt.err.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.err.Type)
			}
			if got.Param != tt.err.Param {
				t.Errorf("Param = %q, want %q", got.Param, tt.err.Param)
			}
			if got.Message != tt.err.Message {
				t.Errorf("Message = %q, want %q", got.Message, tt.err.Message)
			}
		})
	}
}

func TestErrorResponseJSON(t *testing.T) {
	resp := ErrorResponse{Error: NewInvalidRequestError("model", "is required")}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ErrorResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Error.Type != ErrorTypeInvalidRequest {
		t.Errorf("Error.Type = %q, want %q", got.Error.Type, ErrorTypeInvalidRequest)
	}
}

func TestAPIErrorOmitEmpty(t *testing.T) {
	err := &APIError{Type: ErrorTypeServerError, Message: "fail"}
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("Marshal: %v", marshalErr)
	}

	var m map[string]interface{}
	if unmarshalErr := json.Unmarshal(data, &m); unmarshalErr != nil {
		t.Fatalf("Unmarshal: %v", unmarshalErr)
	}

	if _, ok := m["code"]; ok {
		t.Error("empty code should be omitted from JSON")
	}
	if _, ok := m["param"]; ok {
		t.Error("empty param should be omitted from JSON")
	}
}
