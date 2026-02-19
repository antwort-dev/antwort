package auth

import (
	"context"
	"sync"
	"time"
)

// RateLimiter checks whether a request should be allowed based on
// the identity's service tier.
type RateLimiter interface {
	Allow(ctx context.Context, identity *Identity) error
}

// TierConfig holds rate limit settings for a service tier.
type TierConfig struct {
	RequestsPerMinute int
}

// InProcessLimiter is a simple sliding-window rate limiter that tracks
// request counts per subject in memory.
type InProcessLimiter struct {
	tiers      map[string]TierConfig
	defaultRPM int
	mu         sync.Mutex
	counters   map[string]*counter
}

type counter struct {
	count    int
	windowAt time.Time
}

// NewInProcessLimiter creates a rate limiter with per-tier configuration.
func NewInProcessLimiter(tiers map[string]TierConfig, defaultRPM int) *InProcessLimiter {
	return &InProcessLimiter{
		tiers:      tiers,
		defaultRPM: defaultRPM,
		counters:   make(map[string]*counter),
	}
}

// Allow checks if the request is within the rate limit.
// Fails open: any internal error allows the request.
func (l *InProcessLimiter) Allow(_ context.Context, identity *Identity) error {
	tier := identity.ServiceTier
	if tier == "" {
		tier = "default"
	}

	rpm := l.defaultRPM
	if tc, ok := l.tiers[tier]; ok {
		rpm = tc.RequestsPerMinute
	}

	if rpm <= 0 {
		return nil // no limit
	}

	key := identity.Subject + ":" + tier

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	c, ok := l.counters[key]
	if !ok || now.Sub(c.windowAt) >= time.Minute {
		// New window.
		l.counters[key] = &counter{count: 1, windowAt: now}
		return nil
	}

	c.count++
	if c.count > rpm {
		return ErrTooManyRequests
	}

	return nil
}
