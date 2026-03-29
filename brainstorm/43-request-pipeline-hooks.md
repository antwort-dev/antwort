# Brainstorm 43: Request Pipeline Hooks

**Date**: 2026-03-21
**Participants**: Roland Huss
**Inspiration**: SMG's WebAssembly plugin system with OnRequest/OnResponse hooks
**Goal**: Evaluate whether Antwort needs a request pipeline hook system for custom middleware logic, and if so, what form it should take.

## Motivation

SMG allows operators to inject custom logic at two points in the request lifecycle via WebAssembly plugins:
- **OnRequest**: Before forwarding to the backend (auth, validation, transformation, rate limiting)
- **OnResponse**: After receiving the backend response (transformation, error normalization, logging)

Plugins can **Continue**, **Reject**, or **Modify** the request/response. They're loaded dynamically via a REST API without restart.

Antwort currently handles cross-cutting concerns via hardcoded middleware chains:
- Auth middleware (spec 007)
- Scope middleware (spec 041)
- Audit logging (spec 042)
- Tenant extraction

This works for built-in features but doesn't allow operators to add custom logic without forking Antwort.

## Use Cases for Hooks in Antwort

### Concrete scenarios where operators want custom logic:

1. **Content filtering / guardrails**: Block requests containing prohibited content, PII, or prompt injection patterns. This is the #1 enterprise concern that Antwort doesn't address yet.

2. **Request enrichment**: Add default tools, system instructions, or metadata based on the requesting user/tenant. Agent profiles (spec 038) handle some of this, but operators may want dynamic logic.

3. **Response transformation**: Strip internal metadata, add compliance headers, or transform error responses to match organization-specific formats.

4. **Custom rate limiting**: Per-user, per-model, or token-budget-based rate limiting beyond what Kubernetes ingress provides.

5. **Cost tracking**: Intercept token usage from responses and emit to a billing/chargeback system.

6. **Model routing**: Override the requested model based on user tier, time of day, or A/B testing configuration. (Note: this moves Antwort toward SMG territory, which may not be desirable.)

7. **Request/response logging**: Capture full request/response pairs to a separate system for compliance, debugging, or fine-tuning data collection.

## Design Options

### Option A: Go Plugin Interface (recommended)

Define a Go interface that custom plugins implement. Plugins are compiled into the Antwort binary (or loaded via Go plugins, though that's fragile).

```go
// pkg/middleware/hook.go
type Hook interface {
    Name() string
    OnRequest(ctx context.Context, req *api.CreateResponseRequest) (Action, error)
    OnResponse(ctx context.Context, req *api.CreateResponseRequest, resp *api.Response) (Action, error)
}

type Action int
const (
    Continue Action = iota
    Reject          // returns error to client
    Modify          // request/response was mutated in place
)

type HookChain struct {
    hooks []Hook
}

func (c *HookChain) RunOnRequest(ctx context.Context, req *api.CreateResponseRequest) error { ... }
func (c *HookChain) RunOnResponse(ctx context.Context, req *api.CreateResponseRequest, resp *api.Response) error { ... }
```

**Pros**: Type-safe, no runtime overhead, easy to test, stdlib-only
**Cons**: Requires recompilation to add hooks, not dynamically loadable

### Option B: WebAssembly Plugins (like SMG)

Use a WASM runtime (e.g., wazero, which is pure Go) to execute sandboxed plugins.

```yaml
hooks:
  - name: content-filter
    wasm: /etc/antwort/plugins/content-filter.wasm
    attach: [on_request]
  - name: cost-tracker
    wasm: /etc/antwort/plugins/cost-tracker.wasm
    attach: [on_response]
```

**Pros**: Language-agnostic, sandboxed, dynamically loadable, no restart needed
**Cons**: New dependency (wazero), serialization overhead, more complex debugging, WASM component model is still maturing

### Option C: HTTP Webhook Hooks

Call external HTTP endpoints at hook points. Similar to Kubernetes admission webhooks.

```yaml
hooks:
  on_request:
    - url: https://guardrails.internal/check
      timeout: 500ms
      fail_open: false
  on_response:
    - url: https://billing.internal/track
      timeout: 200ms
      fail_open: true
```

**Pros**: Language-agnostic, no binary changes, familiar pattern (K8s admission webhooks)
**Cons**: Network latency per request, availability dependency, debugging complexity

### Option D: Expression-Based Rules (lightweight)

Simple rule engine using CEL (Common Expression Language) or similar:

```yaml
hooks:
  on_request:
    - name: block-large-requests
      condition: "size(request.input) > 100000"
      action: reject
      message: "Request too large"
    - name: force-model
      condition: "identity.role == 'free-tier'"
      action: modify
      set:
        model: "small-model"
```

**Pros**: No code needed, declarative, easy to understand
**Cons**: Limited expressiveness, new dependency (CEL evaluator), can't handle complex logic

## Recommendation

**Phase 1: Go Plugin Interface (Option A) for built-in hooks**

This is the pragmatic choice that aligns with Antwort's constitution:
- Refactor existing middleware (auth, scope, audit) as Hook implementations
- Add a `HookChain` that runs before/after the engine processes a request
- Ship a few useful built-in hooks: content length limit, model allowlist, request/response logging
- The interface is the extension point; operators who need custom logic compile a custom build

This is already how the existing middleware works, just formalized into a consistent interface.

**Phase 2: Webhook Hooks (Option C) for external integration**

For operators who can't recompile:
- HTTP callouts to external services at hook points
- Kubernetes-native pattern (admission webhooks)
- `fail_open` / `fail_closed` configuration per hook
- Timeout enforcement to prevent hook latency from dominating request time

**Phase 3: WASM (Option B) only if demand materializes**

WASM plugins are powerful but add significant complexity. Only pursue if:
- Multiple operators need custom in-process logic
- The webhook approach has unacceptable latency
- The WASM component model matures further

## Hook Points in Antwort's Request Lifecycle

```
Client Request
  │
  ├─ [OnRequest hooks]           ← NEW: content filter, rate limit, enrichment
  │
  ├─ Auth middleware              (existing, could become a hook)
  ├─ Scope middleware             (existing, could become a hook)
  ├─ Audit logging                (existing, could become a hook)
  │
  ├─ Engine: create response
  │    ├─ Rehydrate conversation
  │    ├─ [OnBeforeInference]     ← NEW: modify prompt before sending to backend
  │    ├─ Provider: call backend
  │    ├─ [OnAfterInference]      ← NEW: inspect/modify raw backend response
  │    ├─ Tool execution
  │    └─ (loop if agentic)
  │
  ├─ [OnResponse hooks]          ← NEW: cost tracking, response transformation
  │
  └─ Client Response
```

The four hook points (OnRequest, OnBeforeInference, OnAfterInference, OnResponse) cover the full lifecycle. Start with just OnRequest and OnResponse (matching SMG's model).

## What NOT to Copy from SMG

1. **Dynamic loading via REST API**: Antwort is Kubernetes-native. Config changes should come from ConfigMaps/Secrets, not runtime API calls. This aligns with GitOps practices.
2. **WASM as the primary extensibility mechanism**: Too heavy for Antwort's scope. Go interfaces are simpler and more maintainable.
3. **Plugin marketplace**: SMG's model of downloadable plugins doesn't fit Antwort's focused scope.

## Relation to Existing Features

- **Auth middleware** (spec 007): Already a hook-like pattern. Could be formalized as a Hook implementation.
- **Scope middleware** (spec 041): Same.
- **Audit logging** (spec 042): The OnRequest/OnResponse pattern naturally integrates with audit events.
- **Agent profiles** (spec 038): Profile-based request enrichment overlaps with OnRequest hooks. Profiles handle the common case; hooks handle the edge cases.
- **Guardrails**: This is the most requested missing feature. An OnRequest hook is the natural place for content safety checks. Could be a built-in hook that calls an external guardrails service (like Granite Guardian or NVIDIA NeMo Guardrails).

## Spec Potential

This brainstorm could yield two specs:
1. **Hook Interface + Built-in Hooks**: Formalize the middleware chain, add content length limit and model allowlist hooks
2. **Guardrails Hook**: Specific hook for content safety with pluggable backends (Granite Guardian, regex patterns, keyword blocklist)

## Open Questions

1. Should hooks run inside or outside the auth boundary? (SMG runs WASM plugins after auth.)
2. How should hook errors interact with audit logging? (Every rejection should be auditable.)
3. Should hooks have access to the full conversation history, or only the current request?
4. For streaming responses, should OnResponse hooks see the complete response (buffered) or individual SSE events?
