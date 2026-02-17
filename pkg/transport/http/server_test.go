package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	gohttp "net/http"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/transport"
)

type testServerCreator struct {
	response *api.Response
}

func (c *testServerCreator) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error {
	return w.WriteResponse(ctx, c.response)
}

func jsonBody(t *testing.T, v any) io.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	return bytes.NewReader(data)
}

func TestServerStartsAndAcceptsRequests(t *testing.T) {
	creator := &testServerCreator{
		response: &api.Response{
			ID:     "resp_serverTestABCD567890123",
			Object: "response",
			Status: api.ResponseStatusCompleted,
			Model:  "test-model",
		},
	}

	srv := NewServer(creator, nil, WithAddr("127.0.0.1:0"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := ln.Addr().String()

	go srv.ServeOn(ln)
	time.Sleep(50 * time.Millisecond)

	resp, err := gohttp.Post("http://"+addr+"/v1/responses", "application/json",
		jsonBody(t, api.CreateResponseRequest{Model: "test", Input: []api.Item{{Type: api.ItemTypeMessage}}}))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != gohttp.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, gohttp.StatusOK)
	}

	var got api.Response
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != "resp_serverTestABCD567890123" {
		t.Errorf("response ID = %q, want %q", got.ID, "resp_serverTestABCD567890123")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func TestServerGracefulShutdown(t *testing.T) {
	slowCreator := transport.ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return w.WriteResponse(ctx, &api.Response{
				ID:     "resp_gracefulTestABCD5678901",
				Status: api.ResponseStatusCompleted,
			})
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	srv := NewServer(slowCreator, nil,
		WithAddr("127.0.0.1:0"),
		WithShutdownTimeout(5*time.Second),
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := ln.Addr().String()

	go srv.ServeOn(ln)
	time.Sleep(50 * time.Millisecond)

	responseCh := make(chan int, 1)
	go func() {
		resp, err := gohttp.Post("http://"+addr+"/v1/responses", "application/json",
			jsonBody(t, api.CreateResponseRequest{Model: "test", Input: []api.Item{{Type: api.ItemTypeMessage}}}))
		if err != nil {
			responseCh <- 0
			return
		}
		defer resp.Body.Close()
		responseCh <- resp.StatusCode
	}()

	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	status := <-responseCh
	if status != gohttp.StatusOK {
		t.Errorf("slow request status = %d, want %d", status, gohttp.StatusOK)
	}
}

func TestServerFunctionalOptions(t *testing.T) {
	srv := NewServer(&testServerCreator{}, nil,
		WithAddr(":9999"),
		WithMaxBodySize(1024),
		WithShutdownTimeout(10*time.Second),
	)

	if srv.config.Addr != ":9999" {
		t.Errorf("addr = %q, want %q", srv.config.Addr, ":9999")
	}
	if srv.config.MaxBodySize != 1024 {
		t.Errorf("max body size = %d, want %d", srv.config.MaxBodySize, 1024)
	}
	if srv.config.ShutdownTimeout != 10*time.Second {
		t.Errorf("shutdown timeout = %v, want %v", srv.config.ShutdownTimeout, 10*time.Second)
	}
}
