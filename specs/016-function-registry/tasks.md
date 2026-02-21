# Tasks: Function Provider Registry

## Phase 1: Interface + Registry (P1)

- [ ] T001 [US1] Create `pkg/tools/registry/provider.go`: FunctionProvider interface (Name, Tools, CanExecute, Execute, Routes, Collectors, Close) and Route struct (Method, Pattern, Handler) (FR-001 to FR-004).
- [ ] T002 [US1] Create `pkg/tools/registry/registry.go`: FunctionRegistry implementing ToolExecutor. Register(provider), DiscoveredTools(), CanExecute(), Execute() routing to correct provider. Kind() returns new ToolKindBuiltin. Panic recovery in Execute (FR-005 to FR-008, FR-018, FR-019).
- [ ] T003 [US1] Write tests in `pkg/tools/registry/registry_test.go`: mock provider, tool discovery, execution routing, tool name conflict (first wins), panic recovery, disabled provider.

**Checkpoint**: Registry works as ToolExecutor.

---

## Phase 2: Infrastructure Wrapping (P1)

- [ ] T004 [US2] Create `pkg/tools/registry/middleware.go`: wrapProviderRoutes() that wraps each provider's routes with auth middleware + metrics middleware + tenant injection. Record antwort_builtin_api_requests_total and antwort_builtin_api_duration_seconds (FR-009, FR-011, FR-013).
- [ ] T005 [US2] Add automatic execution metrics to registry.go: record antwort_builtin_tool_executions_total and antwort_builtin_tool_duration_seconds on every Execute call (FR-010).
- [ ] T006 [US2] [US3] Implement HTTPHandler() on registry that merges all wrapped provider routes. Register provider custom collectors with Prometheus registry (FR-008, FR-012).
- [ ] T007 [US2] Write middleware tests: auth bypass without credentials fails, metrics recorded, tenant propagated.

**Checkpoint**: Provider routes have auth + metrics automatically.

---

## Phase 3: Config + Server Integration

- [ ] T008 Add `Providers map[string]ProviderConfig` to config struct in `pkg/config/config.go`. ProviderConfig has Enabled bool + Settings map[string]any for provider-specific config (FR-014 to FR-017).
- [ ] T009 Wire registry into `cmd/server/main.go`: read providers config, create registry, register with engine executors, mount HTTPHandler on server mux.
- [ ] T010 [P] Run `go vet ./...` and `go test ./...`.

---

## Dependencies

- Phase 1: No dependencies.
- Phase 2: Depends on Phase 1.
- Phase 3: Depends on Phase 2.
