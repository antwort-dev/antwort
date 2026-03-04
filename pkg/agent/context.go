package agent

import "context"

// vectorStoreIDsKey is a private type for the profile vector store IDs context key.
type vectorStoreIDsKey struct{}

// SetVectorStoreIDs injects profile-level vector store IDs into the context.
// These are merged with any request-level vector store IDs by the file_search tool.
func SetVectorStoreIDs(ctx context.Context, ids []string) context.Context {
	return context.WithValue(ctx, vectorStoreIDsKey{}, ids)
}

// GetVectorStoreIDs extracts profile-level vector store IDs from the context.
// Returns nil if no IDs are set.
func GetVectorStoreIDs(ctx context.Context) []string {
	if v, ok := ctx.Value(vectorStoreIDsKey{}).([]string); ok {
		return v
	}
	return nil
}
