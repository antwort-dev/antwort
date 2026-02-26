# Brainstorm 29: OpenResponses Conformance CI Integration

## Source

[Slack message](https://redhat-internal.slack.com/archives/C08CD63RDLG/p1772058503581889) from Charlie Doern: Llama Stack achieved 100% OpenResponses conformance (6/6 tests) via [PR #4999](https://github.com/llamastack/llama-stack/pull/4999). They run the conformance tests on every PR as a blocking check using the [OpenResponses CLI](https://github.com/openresponses/openresponses).

## What Llama Stack Does

Their [CI workflow](https://github.com/llamastack/llama-stack/actions/workflows/openresponses-conformance.yml) runs on every PR:

1. Start Llama Stack server in "replay mode" (deterministic, recorded responses)
2. Clone the official openresponses repo
3. Install bun + dependencies
4. Run `bun run bin/compliance-test.ts --base-url http://localhost:8321/v1 --model openai/gpt-4o-mini`
5. Parse JSON results, generate GitHub summary markdown
6. Upload results as artifacts

The check is blocking: failing tests prevent PR merge.

## The 6 Official Tests

The [OpenResponses compliance suite](https://github.com/openresponses/openresponses/blob/main/src/lib/compliance-tests.ts) has 6 tests:

| Test | What It Validates |
|---|---|
| basic-response | Simple user message, response schema (Zod ResponseResource) |
| streaming-response | SSE events, streaming event schema validation |
| system-prompt | System role message in conversation input |
| tool-calling | Function tool definition, function_call output type |
| image-input | Image URL in user content (input_image type) |
| multi-turn | Assistant + user messages as conversation history |

Each test:
- Sends a request body validated against `createResponseBodySchema`
- Validates the response against `responseResourceSchema` (Zod)
- Runs semantic validators: `hasOutput`, `hasOutputType`, `completedStatus`, `streamingEvents`, `streamingSchema`

## What Antwort Already Has

Antwort has three layers of conformance testing:

### Layer 1: `make conformance` (Official OpenResponses Zod suite)
- Builds a container with the official openresponses compliance suite
- Runs against antwort + mock-backend
- Core profile: 5/6 tests (excludes image-input)
- Extended profile: all 6 tests
- Uses `conformance/run.sh` and `conformance/Containerfile`
- Currently run manually, not in CI

### Layer 2: `make api-test` (oasdiff + integration tests)
- oasdiff validates our OpenAPI spec against the upstream OpenResponses spec
- 50+ Go integration tests via httptest
- Run manually via `test/run.sh`

### Layer 3: Per-spec integration tests
- Each spec adds tests to `test/integration/responses_test.go`
- Covers endpoints, streaming, tools, reasoning, structured output, list, input_items
- Run via `go test ./test/integration/`

## The Gap

Antwort already has `make conformance` that does the same thing as Llama Stack's CI. The gap is that it's not in CI. We need a GitHub Actions workflow (or equivalent) that:

1. Runs `make conformance PROFILE=core` on every PR
2. Reports results as a GitHub check
3. Blocks merge on failure

The infrastructure exists. The workflow just needs to be wired.

## Proposal

### Option A: Minimal (wire existing tooling to CI)

Add a GitHub Actions workflow that:
1. Checks out code
2. Installs Go 1.22+, podman, bun
3. Runs `make conformance PROFILE=core`
4. Fails the check if any test fails

This is the fastest path. No new code. Just a workflow file.

### Option B: Match Llama Stack exactly

Skip the container and run the compliance suite directly:
1. Clone openresponses repo
2. `bun install` + `bun run bin/compliance-test.ts`
3. Build and start antwort + mock-backend
4. Run against localhost

This avoids the container build step (faster CI) but adds bun as a dependency.

### Option C: Expand to include `make api-test`

Run both:
1. `make api-test` (oasdiff + integration tests)
2. `make conformance PROFILE=core` (official compliance suite)

This validates both our own contract and the official OpenResponses spec.

## Existing Spec Check

- **Spec 006 (Conformance)**: Created the original conformance infrastructure. Covers the Zod suite setup, profiles, container image. This spec is fully implemented.
- **Spec 019 (API Conformance)**: Created the oasdiff pipeline and `make api-test`. Also fully implemented.

Neither spec covers CI integration. This is a new concern.

## Recommendation

Create a new brainstorm/spec specifically for CI integration rather than expanding spec 006 or 019. Those specs are about the testing tools themselves. CI integration is about the workflow that runs them.

Alternatively, this could be a simple ops task (just add a workflow file) that doesn't need a full spec. The conformance infrastructure is mature. The only thing missing is the `.github/workflows/conformance.yml` file.

## Complexity Assessment

Very low. This is a workflow YAML file, not a code change. The hardest part is ensuring podman works in GitHub Actions (or switching to the direct bun approach).

## Next Steps

1. Decide: Full spec or just do it?
2. Decide: Option A (container via podman), B (direct bun), or C (both pipelines)?
3. Write the workflow file
4. Verify it passes on a PR
5. Make it a blocking check
