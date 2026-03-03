# Feature Specification: Agent Profiles & Prompt Templates

**Feature Branch**: `038-agent-profiles`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "Agent Profiles with OpenAI prompt parameter compatibility. Server-side agent configurations with template variables, plus a compatibility shim for the OpenAI prompt parameter on the Responses API."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Define and Use an Agent Profile (Priority: P1)

An administrator defines agent profiles in the antwort configuration file. Each profile specifies a model, instructions (with optional `{{variable}}` placeholders), tools, and constraints. A client sends a request with `"agent": "profile-name"` and the system resolves the profile, merging its settings with the request. The client does not need to repeat model, instructions, or tool configuration on every request.

**Why this priority**: Agent profiles are the core value. They transform antwort from "send everything every time" to "define once, use by name." This is the minimum feature that reduces client complexity and enables reusable agent behaviors.

**Independent Test**: Define a profile in config with model, instructions, and tools. Send a request with `"agent": "profile-name"` and no model/instructions/tools. Verify the response uses the profile's model and follows the profile's instructions.

**Acceptance Scenarios**:

1. **Given** a profile "devops-helper" defined in config with model, instructions, and tools, **When** a client sends `{"agent": "devops-helper", "input": [...]}`, **Then** the request is processed using the profile's model, instructions, and tools
2. **Given** a profile with instructions containing `{{project_name}}`, **When** a client sends `{"agent": "profile-name", "input": [...]}` with variables, **Then** the template variables are substituted before the instructions reach the LLM
3. **Given** a request with both `"agent"` and explicit `"model"`, **When** processed, **Then** the request's explicit model overrides the profile's model (request fields take precedence)
4. **Given** an `"agent"` value that does not match any profile, **When** processed, **Then** the request fails with a clear error identifying the unknown profile name

---

### User Story 2 - OpenAI Prompt Parameter Compatibility (Priority: P1)

A client using the standard OpenAI SDK sends a request with the `prompt` parameter (as defined in the OpenAI Responses API). The system resolves the prompt against stored profiles, substitutes variables, and uses the result as instructions. This enables clients built for the OpenAI Prompts feature to work with antwort without modification.

**Why this priority**: OpenAI SDK compatibility is a core project goal. The `prompt` parameter is part of the official Responses API. Supporting it ensures clients using `prompt: {id: "...", variables: {...}}` work out of the box.

**Independent Test**: Define a profile. Send a request with `"prompt": {"id": "profile-id", "variables": {"name": "Alice"}}`. Verify the response uses the resolved instructions with "Alice" substituted.

**Acceptance Scenarios**:

1. **Given** a profile with ID "pmpt_abc123" and instructions containing `{{customer_name}}`, **When** a client sends `{"prompt": {"id": "pmpt_abc123", "variables": {"customer_name": "Jane"}}}`, **Then** the instructions are resolved with "Jane" substituted
2. **Given** a prompt request with `"version"` specified, **When** processed, **Then** the specified version of the profile is used (if versioning is supported) or the current version is used
3. **Given** a prompt ID that does not match any profile, **When** processed, **Then** the request fails with a 404-style error

---

### User Story 3 - Profile Merge Logic (Priority: P1)

When a request includes an agent profile reference, the profile's settings are merged with the request's explicit settings. Request-level settings always take precedence over profile defaults. This ensures profiles provide sensible defaults while allowing per-request overrides.

**Why this priority**: Without clear merge semantics, the interaction between profiles and request fields would be ambiguous and error-prone.

**Independent Test**: Define a profile with temperature=0.3. Send a request with `"agent": "profile-name"` and `"temperature": 0.7`. Verify the response uses temperature 0.7 (request wins).

**Acceptance Scenarios**:

1. **Given** a profile with `model: "model-a"` and a request with no explicit model, **When** processed, **Then** the profile's model is used
2. **Given** a profile with `model: "model-a"` and a request with `"model": "model-b"`, **When** processed, **Then** the request's model-b is used (request wins)
3. **Given** a profile with tools `[web_search]` and a request with tools `[code_interpreter]`, **When** processed, **Then** both tool sets are merged (profile tools + request tools)
4. **Given** a profile with `temperature: 0.3` and a request with no temperature, **When** processed, **Then** the profile's temperature is used
5. **Given** a request with no `agent` field, **When** processed, **Then** the request is handled exactly as before (no profile resolution, backward compatible)

---

### User Story 4 - List Available Profiles (Priority: P2)

A client can discover which agent profiles are available by listing them. This enables UIs to show a profile picker and clients to validate profile names before sending requests.

**Why this priority**: Discovery is important for usability but not for core functionality. Clients can use profiles by name without listing them first.

**Independent Test**: Define three profiles in config. Call the list endpoint. Verify all three are returned with their names and descriptions.

**Acceptance Scenarios**:

1. **Given** multiple profiles defined in config, **When** a client lists profiles, **Then** all profiles are returned with their name, description, and model
2. **Given** a profile list request, **When** processed, **Then** the response does not include full instructions or tool definitions (summary only, for security and size)

---

### Edge Cases

- What happens when both `agent` and `prompt` are set on the same request? The `agent` field takes precedence. The `prompt` field is ignored with a warning.
- What happens when a profile references a tool that is not configured? The tool is silently skipped (same as requesting an unconfigured tool directly).
- What happens when template variables are referenced in instructions but not provided in the request? The `{{variable}}` placeholder remains in the text (no error, the LLM sees it as literal text). This follows Mustache convention.
- What happens when the config file is reloaded and profiles change? Currently, profiles are loaded at startup. Hot-reloading is out of scope (future enhancement).

## Requirements *(mandatory)*

### Functional Requirements

**Agent Profiles**

- **FR-001**: The system MUST support defining agent profiles in the configuration file with: name (required), model (optional), instructions (optional, supports `{{variable}}` templates), tools (optional), and constraints (optional: temperature, max_output_tokens, max_tool_calls, reasoning)
- **FR-002**: The system MUST accept an optional `agent` field on the create response request that references a profile by name
- **FR-003**: When `agent` is set, the system MUST resolve the profile and merge its settings with the request using the merge logic defined in FR-006
- **FR-004**: The system MUST return a clear error when the `agent` field references a non-existent profile

**Prompt Parameter (OpenAI Compatibility)**

- **FR-005**: The system MUST accept an optional `prompt` parameter on the create response request with fields: `id` (required), `version` (optional, ignored in v1), and `variables` (optional map of string substitutions)
- **FR-006**: When `prompt` is set, the system MUST resolve the profile by ID, substitute `{{variable}}` placeholders in the instructions with the provided variables, and use the result as the request's instructions
- **FR-007**: The `prompt` parameter MUST work with standard OpenAI SDKs without modification

**Merge Logic**

- **FR-008**: Request-level fields MUST take precedence over profile defaults for: model, instructions, temperature, top_p, max_output_tokens, max_tool_calls, reasoning
- **FR-009**: Tools from the profile and tools from the request MUST be merged (union, not replacement)
- **FR-010**: When no `agent` or `prompt` is set, the request MUST be processed exactly as before (full backward compatibility)

**Template Variables**

- **FR-011**: The system MUST support `{{variable_name}}` syntax for template substitution in profile instructions
- **FR-012**: Variables provided via the `prompt.variables` field or via a new `variables` field on the request MUST be substituted before the instructions are sent to the LLM
- **FR-013**: Undefined variables (referenced in template but not provided) MUST be left as literal text (no error)

**Profile Discovery**

- **FR-014**: The system MUST provide an endpoint to list available profiles with summary information (name, description, model)
- **FR-015**: The profile list MUST NOT expose full instructions or tool configurations (summary only)

**Configuration**

- **FR-016**: Profiles MUST be definable in the existing configuration file under an `agents` section
- **FR-017**: Each profile MUST have a unique name within the configuration
- **FR-018**: The system MUST validate profile definitions at startup and fail with clear errors for invalid configurations

**Documentation (per constitution v1.6.0)**

- **FR-019**: The feature MUST include an API reference page documenting the `agent` field, `prompt` parameter, and profile list endpoint
- **FR-020**: The feature MUST include a tutorial page showing how to define profiles and use them from OpenAI SDKs
- **FR-021**: The feature MUST include a developer documentation page explaining the merge logic and template syntax

### Key Entities

- **AgentProfile**: A named configuration bundle with optional model, instructions (template), tools, and constraints. Loaded from config at startup.
- **TemplateVariable**: A `{{name}}` placeholder in profile instructions, resolved at request time from the `variables` map.
- **PromptReference**: The OpenAI `prompt` parameter object with `id`, `version`, and `variables` fields. Maps to an agent profile.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can define an agent profile and use it by name in under 5 minutes (config edit + restart + send request)
- **SC-002**: OpenAI SDK clients can use the `prompt` parameter without code changes or custom configuration
- **SC-003**: Request-level overrides always take precedence over profile defaults (100% deterministic merge)
- **SC-004**: Template variable substitution completes in under 1ms for typical instruction lengths
- **SC-005**: Existing clients without `agent` or `prompt` fields experience zero behavioral changes
- **SC-006**: Documentation covers profile definition, usage from SDKs, merge semantics, and template syntax

## Assumptions

- Profiles are loaded from the config file at startup (no runtime CRUD via API in v1)
- The `prompt` parameter's `version` field is accepted but ignored in v1 (all profiles have one version, the current config)
- Profile names are case-sensitive strings (no normalization)
- The `prompt.id` can match either a profile name or a profile's assigned ID (if one is configured)
- Template syntax uses double curly braces `{{variable}}` only (no conditionals, loops, or filters)

## Dependencies

- **Spec 012 (Configuration)**: Configuration system for loading agent profiles
- **Spec 003 (Core Engine)**: Engine where profile resolution and merge happen
- **Spec 007 (Auth)**: User identity (profiles are global, not user-scoped, but request scoping still applies)

## Scope Boundaries

### In Scope

- Agent profile definitions in configuration file
- `agent` field on create response request
- OpenAI `prompt` parameter compatibility on create response request
- Profile-to-request merge logic (request wins)
- `{{variable}}` template substitution in instructions
- Profile listing endpoint (summary only)
- Startup validation of profile definitions
- API reference, tutorial, and developer documentation

### Out of Scope

- Runtime CRUD for profiles via API (profiles are config-file-only in v1)
- Profile versioning (future enhancement, `version` field accepted but ignored)
- Agent CRDs for Kubernetes (separate future spec)
- Per-user or per-tenant profile scoping (profiles are global)
- Conditional or loop template syntax (simple substitution only)
- Profile hot-reloading on config change (requires restart)
