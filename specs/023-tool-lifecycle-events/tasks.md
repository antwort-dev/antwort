# Tasks: Tool Lifecycle SSE Events

## Phase 1: Event Type Definitions

**Purpose**: Add new SSE event type constants and serialization.

- [x] T001 Add 9 tool lifecycle event type constants to `pkg/api/events.go`: `mcp_call.in_progress`, `mcp_call.completed`, `mcp_call.failed`, `file_search_call.in_progress`, `file_search_call.searching`, `file_search_call.completed`, `web_search_call.in_progress`, `web_search_call.searching`, `web_search_call.completed`. Add MarshalJSON cases (all use item_id + output_index + tool name pattern) (FR-001 through FR-005)
- [x] T002 [P] Add the 9 new event types to the `StreamEventType` enum in `api/openapi.yaml` (FR-012, FR-013)

**Checkpoint**: New event types compile and serialize correctly.

---

## Phase 2: US1/US2/US3 - Tool Lifecycle Emission (P1)

**Goal**: The streaming agentic loop emits lifecycle events around each tool call.

**Independent Test**: Send a streaming agentic request with tools, verify lifecycle events are emitted.

- [x] T003 [US1] Add a helper function `classifyToolType(toolName string, executor ToolExecutor) string` in `pkg/engine/loop.go` that returns "mcp", "file_search", "web_search", or "function" based on the executor type and tool name (FR-002, FR-003, FR-004)
- [x] T004 [US1] Add a new `executeToolsWithEvents()` in `pkg/engine/loop.go` that wraps `executeTools()` with lifecycle event emission. Accepts a `transport.ResponseWriter` and `*streamState` (both non-nil only in streaming mode). Before each tool execution, emit the appropriate `in_progress` event. For search tools, also emit `searching`. After execution, emit `completed` or `failed`. The existing `executeTools()` signature stays unchanged for non-streaming callers (nil-safe composition) (FR-006, FR-007, FR-008)
- [x] T005 [US1] Update `runAgenticLoopStreaming()` in `pkg/engine/loop.go` to call `executeToolsWithEvents()` instead of `executeTools()`, passing the writer and state for lifecycle event emission (FR-001)
- [x] T006 [US1] Verify `runAgenticLoop()` (non-streaming) continues to call `executeTools()` directly without lifecycle events (FR-009)
- [x] T007 [US1] Add a mock tool executor to the integration test environment in `test/integration/helpers_test.go` that handles a test tool (e.g., "mock_search")
- [x] T008 [US1] Add integration test `TestStreamingToolLifecycleEvents` in `test/integration/streaming_test.go`: verify in_progress and completed events are emitted around tool execution (FR-001, FR-009, FR-010)

**Checkpoint**: Tool lifecycle events work end-to-end.

---

## Phase 3: Polish & Validation

- [x] T009 Run `make api-test` and verify all existing tests pass with zero regressions (FR-009, FR-010, FR-011)
- [x] T010 Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions

**Checkpoint**: All tests green. SSE event count at 30.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: Depends on Phase 1
- Phase 3: Depends on Phase 2
