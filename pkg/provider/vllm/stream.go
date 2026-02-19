package vllm

import (
	"context"
	"io"

	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// parseSSEStream delegates to openaicompat.ParseSSEStream.
func parseSSEStream(ctx context.Context, body io.Reader, ch chan<- provider.ProviderEvent) {
	openaicompat.ParseSSEStream(ctx, body, ch)
}
