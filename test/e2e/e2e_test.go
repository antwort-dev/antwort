//go:build e2e

package e2e

import (
	"os"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var (
	baseURL   string
	apiKey    string
	aliceKey  string
	bobKey    string
	model     string
	auditFile string
)

func TestMain(m *testing.M) {
	baseURL = envOr("ANTWORT_BASE_URL", "http://localhost:8080/v1")
	apiKey = envOr("ANTWORT_API_KEY", "test")
	aliceKey = envOr("ANTWORT_ALICE_KEY", "alice-key")
	bobKey = envOr("ANTWORT_BOB_KEY", "bob-key")
	model = envOr("ANTWORT_MODEL", "mock-model")
	auditFile = envOr("ANTWORT_AUDIT_FILE", "/tmp/audit.log")

	os.Exit(m.Run())
}

func newClient(key string) *openai.Client {
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(key),
	)
	return &client
}

func defaultClient() *openai.Client {
	return newClient(apiKey)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
