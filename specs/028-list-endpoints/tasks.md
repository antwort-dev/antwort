# Tasks: List Responses and Input Items Endpoints

## Phase 1: Storage Interface

**Purpose**: Extend the ResponseStore interface with list and input_items methods.

- [X] T001 (antwort-v77.1) Add `ListResponses(ctx, ListOptions) -> (ListResult, error)` and `GetInputItems(ctx, responseID, ListOptions) -> (ListResult, error)` to the `ResponseStore` interface in `pkg/transport/store.go`. Define `ListOptions` (After, Before, Limit, Model, Order) and `ListResult` (Data, HasMore, FirstID, LastID) types (FR-001, FR-002, FR-007, FR-008, FR-011)
- [X] T002 (antwort-v77.2) Implement `ListResponses` in `pkg/storage/memory/memory.go`: filter by model, sort by created_at, apply cursor pagination, enforce limit (FR-001 through FR-006)
- [X] T003 (antwort-v77.3) Implement `GetInputItems` in `pkg/storage/memory/memory.go`: look up response by ID, return input items with pagination (FR-007 through FR-010)

**Checkpoint**: Storage interface extended, in-memory implementation compiles.

---

## Phase 2: HTTP Handlers

**Goal**: Wire the new endpoints into the HTTP adapter.

- [X] T004 (antwort-13w.1) [US1] Add `handleListResponses` handler in `pkg/transport/http/adapter.go`: parse query params (after, before, limit, model, order), call store.ListResponses, return OpenAI list format JSON. Return 501 if no store (FR-001 through FR-006, FR-012)
- [X] T005 (antwort-13w.2) [US2] Add `handleListInputItems` handler in `pkg/transport/http/adapter.go`: extract response ID from path, parse pagination params, call store.GetInputItems, return list format JSON. Return 404 if not found, 501 if no store (FR-007 through FR-010, FR-012)
- [X] T006 (antwort-13w.3) Register both handlers in the adapter's route setup: `GET /v1/responses` and `GET /v1/responses/{id}/input_items` (FR-001, FR-007)

**Checkpoint**: Both endpoints respond to HTTP requests.

---

## Phase 3: Auth and OpenAPI

- [X] T007 (antwort-6lg.1) [US3] Ensure tenant isolation: pass auth identity to ListResponses and GetInputItems, filter by tenant in the in-memory store (FR-013)
- [X] T008 (antwort-6lg.2) [P] Update `api/openapi.yaml` with both new endpoints, query parameters, and list response schema (FR-014)

**Checkpoint**: Auth works, OpenAPI spec updated.

---

## Phase 4: Testing

- [X] T009 (antwort-7q8.1) Add integration tests in `test/integration/responses_test.go`: list responses (empty, single, multiple, pagination, model filter, ordering), input_items (exists, not found, pagination) (FR-001 through FR-010)
- [X] T010 (antwort-7q8.2) Run full test suite and verify zero regressions
- [X] T011 (antwort-7q8.3) Run `make api-test` to verify conformance

**Checkpoint**: All tests pass.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: Depends on Phase 1
- Phase 3: Depends on Phase 2
- Phase 4: Depends on Phase 2 and 3
