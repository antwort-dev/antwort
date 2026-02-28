# Tasks: MCP Client Integration

**Input**: Design documents from `/specs/011-mcp-client/`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Dependencies + Config)

**Purpose**: Add MCP SDK dependency, create config types.

- [x] T001 Add Go MCP SDK dependency to go.mod. Create `pkg/tools/mcp/` package with `doc.go`.
- [x] T002 [P] Create `pkg/tools/mcp/config.go` with MCPConfig (server list) and MCPServerConfig (name, transport, URL, auth) types (FR-015, FR-016).
- [x] T003 [P] Create `pkg/tools/mcp/auth.go` with MCPAuthProvider interface and StaticKeyAuth implementation (FR-011, FR-012).

**Checkpoint**: Config and auth types ready.

---

## Phase 2: User Story 1 - Tool Discovery (Priority: P1)

**Goal**: Connect to MCP server, handshake, discover tools.

- [x] T004 [US1] Create `pkg/tools/mcp/client.go`: MCPClient struct wrapping the SDK client. Connect method (performs handshake), DiscoverTools method (calls tools/list, caches result), Close method. Use configured MCPAuthProvider for headers (FR-001, FR-002, FR-003).
- [ ] T005 [US1] Write client tests in `pkg/tools/mcp/client_test.go`: mock MCP server (httptest) returning tool list, test handshake, test tool discovery, test unreachable server (graceful error).

**Checkpoint**: MCPClient connects and discovers tools.

---

## Phase 3: User Story 2 - Tool Execution (Priority: P1)

**Goal**: Execute MCP tool calls via ToolExecutor interface.

- [x] T006 [US2] Create `pkg/tools/mcp/executor.go`: MCPExecutor implementing ToolExecutor (Kind=ToolKindMCP, CanExecute checks discovered tools, Execute routes to correct server). Manages multiple MCPClients. Lazy tool discovery on first CanExecute call (FR-005, FR-006, FR-007, FR-008).
- [x] T007 [US2] Implement tool execution in MCPClient: CallTool method that sends tools/call, parses result into ToolResult (FR-004, FR-018).
- [x] T008 [US2] Write executor tests in `pkg/tools/mcp/executor_test.go`: mock MCP server, test tool call routing, test error handling, test CanExecute with discovered tools.

**Checkpoint**: MCP tool calls work through the agentic loop.

---

## Phase 4: User Story 1+2 - Engine Integration (Priority: P1)

**Goal**: Wire MCP executor into the engine and merge tools.

- [x] T009 [US1] [US2] Add MCP tool merging in `pkg/engine/engine.go`: before translateRequest, if MCPExecutor is configured, call DiscoveredTools and merge with request tools (FR-009, FR-010).
- [x] T010 [US1] [US2] Wire MCP executor in `cmd/server/main.go`: read MCP config from env (ANTWORT_MCP_SERVERS JSON), create MCPExecutor, add to engine's Executors list (FR-015).
- [ ] T011 [US2] Write engine integration test: mock MCP server + mock provider, verify agentic loop calls MCP tool and produces final answer (SC-001, SC-005).

**Checkpoint**: Full end-to-end MCP flow works.

---

## Phase 5: User Story 3+4 - Auth + Multi-Server (Priority: P1/P2)

**Goal**: API key auth and multi-server routing.

- [x] T012 [US3] Write auth tests in `pkg/tools/mcp/auth_test.go`: StaticKeyAuth returns correct headers, empty auth returns no headers.
- [x] T013 [US4] Write multi-server routing test: two mock MCP servers with different tools, verify correct routing.

**Checkpoint**: Auth and multi-server work.

---

## Phase 6: Polish

- [x] T014 [P] Run `go vet ./...` and `go test ./...` across all packages.
- [ ] T015 [P] Run `make conformance` to verify no regressions.

---

## Dependencies

- **Phase 1**: No dependencies.
- **Phase 2**: Depends on Phase 1 (config + auth types).
- **Phase 3**: Depends on Phase 2 (client must exist).
- **Phase 4**: Depends on Phase 3 (executor must exist).
- **Phase 5**: Can start after Phase 2 (auth tests independent of execution).
- **Phase 6**: Depends on all.

## Implementation Strategy

### MVP: Phases 1-4

1. SDK + config + auth types
2. Client with tool discovery
3. Executor with tool execution
4. Engine integration
5. **STOP**: MCP agentic loop works end-to-end

### Full: + Phase 5-6

6. Auth tests + multi-server tests
7. Polish + conformance
