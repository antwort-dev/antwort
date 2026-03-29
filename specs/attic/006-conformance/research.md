# Research: OpenResponses Conformance Testing

**Feature**: 006-conformance
**Date**: 2026-02-18

## R1: Compliance Suite Execution Method

**Decision**: Run the official TypeScript suite via a podman container built from the openresponses/openresponses repo.

**Rationale**: The official suite uses bun as its runtime and validates against Zod schemas generated from the OpenAPI spec. Running it in a container avoids requiring bun/node on the host and ensures reproducibility. Podman is the project's mandatory container runtime.

**Alternatives considered**:
- Install bun locally: Adds a host dependency. Not reproducible across environments.
- Reimplement in Go: Risks drift from the canonical schemas. Maintenance burden.
- Use the web UI: Not scriptable. Requires browser and CORS.

## R2: Mock Backend Response Strategy

**Decision**: Pattern-match on request content to return appropriate responses for each compliance test.

**Rationale**: The 6 compliance tests use specific prompts ("Say hello in exactly 3 words", "Count from 1 to 5", etc.). The mock inspects the message content and routes to the appropriate response generator. For unrecognized prompts, a generic completion is returned.

**Implementation approach**:
- Check if tools are present -> return tool call response
- Check if messages contain image content parts -> return image acknowledgment
- Check for specific prompt strings -> return deterministic text
- Default -> generic text completion

## R3: Profile Filtering Approach

**Decision**: Post-hoc filtering. All 6 tests run, but the conformance score only counts tests in the active profile.

**Rationale**: The official suite's `runAllTests` function runs all tests. We cannot selectively skip tests without modifying the suite. Post-hoc filtering lets us use the suite as-is while reporting only the relevant subset.

**Profile definitions**:
- "core": tests 1-4, 6 (basic, streaming, system prompt, tool calling, multi-turn) = 5 tests
- "extended": tests 1-6 (all) = 6 tests

## R4: Server Wiring Architecture

**Decision**: Minimal main.go that reads env vars, creates provider/store/engine/server, and starts listening.

**Rationale**: The full configuration system (Spec 09) is not yet implemented. For conformance testing, environment variables are sufficient. The server reads: `ANTWORT_BACKEND_URL`, `ANTWORT_MODEL`, `ANTWORT_PORT`, `ANTWORT_STORAGE_DSN` (optional).

## R5: Containerfile for Compliance Suite

**Decision**: Build a container image from the openresponses repo that runs the compliance tests via bun.

**Rationale**: The container clones the repo, installs dependencies, and runs `bun run test:compliance`. The antwort URL and API key are passed as environment variables. Results are output to stdout as JSON.
