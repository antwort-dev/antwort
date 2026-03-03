# Tasks: Agent Profiles & Prompt Templates

**Input**: Design documents from `/specs/038-agent-profiles/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks grouped by user story.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 [P] Add AgentProfile type and ProfileResolver interface (1 method: Resolve) in `pkg/agent/profile.go`
- [ ] T002 [P] Add PromptReference type to `pkg/api/types.go`, add `Agent`, `Prompt`, `Variables` fields to CreateResponseRequest
- [ ] T003 [P] Add `AgentProfiles map[string]*AgentProfileConfig` to Config struct in `pkg/config/config.go`

---

## Phase 2: Foundational (Core Logic)

- [ ] T004 Implement template substitution (`{{variable}}` replacement via strings.ReplaceAll) in `pkg/agent/template.go`
- [ ] T005 [P] Implement merge logic (profile defaults + request overrides, tool union) in `pkg/agent/merge.go`
- [ ] T006 Implement ConfigResolver (loads profiles from config map, resolves by name) in `pkg/agent/config.go`
- [ ] T007 Write tests for template substitution in `pkg/agent/template_test.go` (table-driven: single var, multiple vars, undefined var left as-is, no vars, empty template)
- [ ] T008 [P] Write tests for merge logic in `pkg/agent/merge_test.go` (table-driven: model override, model default, tool union, temperature override, no profile)
- [ ] T009 Write tests for ConfigResolver in `pkg/agent/config_test.go` (resolve by name, not found error, empty config)

**Checkpoint**: Agent profile resolution and merge work in isolation.

---

## Phase 3: User Story 1 - Define and Use Profiles (Priority: P1) MVP

- [ ] T010 [US1] Integrate ProfileResolver into engine: resolve `agent` field before provider translation, merge profile into request in `pkg/engine/engine.go`
- [ ] T011 [US1] Parse agent profiles from config and create ConfigResolver in `cmd/server/main.go`
- [ ] T012 [US1] Write integration test: define profile, send request with agent field, verify profile settings applied in `pkg/engine/engine_test.go`

**Checkpoint**: `"agent": "profile-name"` resolves and applies profile settings.

---

## Phase 4: User Story 2 - Prompt Parameter (Priority: P1)

- [ ] T013 [US2] Handle `prompt` parameter in engine: map prompt.id to profile name, extract prompt.variables, resolve and merge in `pkg/engine/engine.go`
- [ ] T014 [US2] Write test for prompt parameter: send request with `prompt: {id, variables}`, verify template substitution in `pkg/engine/engine_test.go`

**Checkpoint**: OpenAI SDK `prompt` parameter works.

---

## Phase 5: User Story 3 - Merge Logic (Priority: P1)

- [ ] T015 [US3] Write tests for merge edge cases: both agent and explicit model, tool union, both agent and prompt (agent wins) in `pkg/engine/engine_test.go`

---

## Phase 6: User Story 4 - List Profiles (Priority: P2)

- [ ] T016 [US4] Implement agent list handler (GET /v1/agents: returns profile summaries) in `pkg/transport/http/agents.go`
- [ ] T017 [US4] Register route in adapter mux in `pkg/transport/http/adapter.go`
- [ ] T018 [US4] Write test for agent listing in `pkg/transport/http/agents_test.go`

---

## Phase 7: Documentation (per constitution v1.6.0)

- [ ] T019 [P] Create agent profiles API reference page in `docs/modules/reference/pages/agent-profiles.adoc`
- [ ] T020 [P] Update existing API reference with agent, prompt, variables fields in `docs/modules/reference/pages/api-reference.adoc`
- [ ] T021 [P] Create agent profiles tutorial page in `docs/modules/tutorial/pages/agent-profiles.adoc`
- [ ] T022 [P] Create developer guide for ProfileResolver extension in `docs/modules/developer/pages/agent-profiles.adoc`
- [ ] T023 Update nav.adoc files for reference, tutorial, and developer modules

---

## Phase 8: Polish

- [ ] T024 Verify `go vet ./pkg/agent/... ./pkg/engine/... ./pkg/transport/http/...` pass
- [ ] T025 Verify `go test ./pkg/agent/... ./pkg/engine/... ./pkg/transport/http/...` pass

---

## Dependencies & Execution Order

- **Phase 1**: No dependencies
- **Phase 2**: Depends on Phase 1
- **US1 (Phase 3)**: Depends on Phase 2 (MVP)
- **US2 (Phase 4)**: Depends on US1
- **US3 (Phase 5)**: Depends on US1+US2
- **US4 (Phase 6)**: Depends on Phase 1 only (independent of engine integration)
- **Docs (Phase 7)**: After US1
- **Polish (Phase 8)**: After all

## Implementation Strategy

### MVP: Define + Use Profiles

1. Phase 1: Types and interfaces (T001-T003)
2. Phase 2: Template, merge, resolver (T004-T009)
3. Phase 3: Engine integration (T010-T012)
4. Validate: `"agent": "profile-name"` works end-to-end
