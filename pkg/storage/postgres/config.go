package postgres

import "time"

// Config holds PostgreSQL connection and behavior settings.
type Config struct {
	// DSN is the PostgreSQL connection string (e.g., "postgres://user:pass@host:5432/db?sslmode=require").
	DSN string

	// MaxConns is the maximum number of connections in the pool (default: 25).
	MaxConns int32

	// MinConns is the minimum number of idle connections maintained (default: 5).
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection before it is
	// closed and replaced (default: 5 minutes).
	MaxConnLifetime time.Duration

	// MigrateOnStart runs schema migrations automatically at startup.
	MigrateOnStart bool
}

// defaults applies default values for unset configuration fields.
func (c *Config) defaults() {
	if c.MaxConns == 0 {
		c.MaxConns = 25
	}
	if c.MinConns == 0 {
		c.MinConns = 5
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = 5 * time.Minute
	}
}
