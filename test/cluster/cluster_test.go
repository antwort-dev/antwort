//go:build cluster

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var (
	antwortURL     string
	vllmURL        string
	apiKey         string
	model          string
	testTimeout    time.Duration
	antwortVersion string
	collector      *ResultCollector
	bfclAll        bool
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestMain(m *testing.M) {
	antwortURL = os.Getenv("CLUSTER_ANTWORT_URL")
	if antwortURL == "" {
		fmt.Println("SKIP: CLUSTER_ANTWORT_URL not set, skipping cluster tests")
		os.Exit(0)
	}

	vllmURL = os.Getenv("CLUSTER_VLLM_URL")
	apiKey = envOr("CLUSTER_API_KEY", "")
	model = envOr("CLUSTER_MODEL", "")
	antwortVersion = envOr("CLUSTER_ANTWORT_VERSION", "unknown")

	timeoutSec, _ := strconv.Atoi(envOr("CLUSTER_TIMEOUT", "120"))
	testTimeout = time.Duration(timeoutSec) * time.Second

	if os.Getenv("CLUSTER_BFCL_ALL") == "true" {
		bfclAll = true
	}

	// Check cluster reachability
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthURL := antwortURL + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		fmt.Printf("SKIP: cannot create health check request: %v\n", err)
		os.Exit(0)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("SKIP: cluster not reachable at %s: %v\n", healthURL, err)
		os.Exit(0)
	}
	resp.Body.Close()

	if model == "" {
		fmt.Println("SKIP: CLUSTER_MODEL not set, cannot run cluster tests without a model")
		os.Exit(0)
	}

	collector = NewResultCollector(model, antwortVersion, "")

	code := m.Run()

	// Write results on teardown
	if err := collector.WriteJSON("test/cluster/results/raw"); err != nil {
		fmt.Printf("WARNING: failed to write results JSON: %v\n", err)
	}

	os.Exit(code)
}

func newAntwortClient() openai.Client {
	opts := []option.RequestOption{
		option.WithBaseURL(antwortURL),
	}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	} else {
		opts = append(opts, option.WithAPIKey("unused"))
	}
	return openai.NewClient(opts...)
}

func newVLLMClient() *openai.Client {
	if vllmURL == "" {
		return nil
	}
	c := openai.NewClient(
		option.WithBaseURL(vllmURL),
		option.WithAPIKey("unused"),
	)
	return &c
}

func skipIfNoVLLM(t *testing.T) {
	t.Helper()
	if vllmURL == "" {
		t.Skip("CLUSTER_VLLM_URL not set, skipping direct vLLM comparison")
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}
