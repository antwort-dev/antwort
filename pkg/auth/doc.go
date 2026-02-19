// Package auth provides pluggable authentication and authorization for antwort.
//
// Authentication uses a chain-of-responsibility pattern with three-outcome
// voting: each authenticator returns Yes (identity found), No (credentials
// invalid), or Abstain (can't handle). A configurable default voter decides
// when all authenticators abstain.
//
// Auth is implemented as HTTP middleware, keeping it decoupled from engine
// logic. The middleware also injects the tenant identity into the request
// context for storage multi-tenancy scoping.
package auth
