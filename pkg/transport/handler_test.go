package transport

import (
	"context"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestResponseCreatorFuncAdapter(t *testing.T) {
	called := false
	var receivedReq *api.CreateResponseRequest

	fn := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		called = true
		receivedReq = req
		return nil
	})

	// Verify it satisfies the interface.
	var _ ResponseCreator = fn

	req := &api.CreateResponseRequest{Model: "test-model"}
	err := fn.CreateResponse(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected function to be called")
	}
	if receivedReq.Model != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", receivedReq.Model)
	}
}

func TestResponseCreatorFuncReturnsError(t *testing.T) {
	fn := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		return api.NewServerError("test error")
	})

	err := fn.CreateResponse(context.Background(), &api.CreateResponseRequest{}, nil)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestInterfaceSatisfaction(t *testing.T) {
	// Compile-time interface checks.
	var _ ResponseCreator = ResponseCreatorFunc(nil)
	var _ ResponseCreator = (*mockCreator)(nil)
	var _ ResponseStore = (*mockStore)(nil)
}

// Mock implementations for compile-time verification.
type mockCreator struct{}

func (m *mockCreator) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
	return nil
}

type mockStore struct{}

func (m *mockStore) SaveResponse(_ context.Context, _ *api.Response) error                  { return nil }
func (m *mockStore) GetResponse(_ context.Context, _ string) (*api.Response, error)         { return nil, nil }
func (m *mockStore) GetResponseForChain(_ context.Context, _ string) (*api.Response, error) { return nil, nil }
func (m *mockStore) DeleteResponse(_ context.Context, _ string) error                       { return nil }
func (m *mockStore) ListResponses(_ context.Context, _ ListOptions) (*ResponseList, error)  { return nil, nil }
func (m *mockStore) GetInputItems(_ context.Context, _ string, _ ListOptions) (*ItemList, error) {
	return nil, nil
}
func (m *mockStore) HealthCheck(_ context.Context) error { return nil }
func (m *mockStore) Close() error                        { return nil }
