# Brainstorm 20: Image Generation, Code Interpreter, and Shell Tool Events

## Problem

The upstream spec defines event types for three advanced tool capabilities that Antwort doesn't yet implement: image generation, code interpreter, and shell execution. These are substantial features, each requiring their own infrastructure.

## Missing SSE Events (12)

### Image generation (4)
- `response.image_gen_call.in_progress`
- `response.image_gen_call.generating`
- `response.image_gen_call.partial_image`
- `response.image_gen_call.completed`
- Plus image editing: `image_edit.completed`, `image_edit.partial_image`

### Shell / Apply Patch (5)
- `response.shell_call.command.added`
- `response.shell_call.command.delta`
- `response.shell_call.command.done`
- `response.apply_patch_call.operation.diff.delta`
- `response.apply_patch_call.operation.diff.done`

## Assessment

These events correspond to features that are significant standalone capabilities:

**Image generation** would require integrating with image generation backends (DALL-E compatible APIs, Stable Diffusion). This would be a new FunctionProvider similar to web_search/file_search, with its own backend configuration.

**Code interpreter** is the closest to what Antwort already plans: the agent-sandbox integration (Spec 011 in brainstorm) would execute code in sandbox pods. The code interpreter events would come from the sandbox execution progress.

**Shell / Apply Patch** are OpenAI-specific tools for Codex-style coding assistants. These map to the sandbox concept but with specific semantics (running shell commands, applying file patches).

## Recommendation

These are not SSE event gaps to close. They are **feature gaps** that would each require their own spec. The events naturally follow once the features exist:

1. **Code interpreter** - covered by brainstorm/11-sandbox.md (agent-sandbox integration)
2. **Image generation** - would need a new brainstorm (image generation provider)
3. **Shell / Apply Patch** - would be a sandbox variant or Codex-compatible tool provider

Don't chase the event count. Build the features; the events come for free.
