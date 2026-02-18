// Package storage provides utilities shared across storage adapter
// implementations, including sentinel errors and tenant context helpers.
//
// Storage adapters (memory, postgres) implement the transport.ResponseStore
// interface defined in pkg/transport/handler.go. This package contains
// only shared types and helpers, not the interface itself.
package storage
