# Feature Specification: Scope-Based Authorization and Resource Permissions

**Feature Branch**: `041-scope-permissions`
**Created**: 2026-03-04
**Status**: Draft
**Input**: User description: "Scope-based endpoint authorization with role-to-scope mapping, three-level resource permissions for vector stores and files, and vector store union merge with agent profiles"

## Clarifications

### Session 2026-03-04

- Q: How should JWT scopes and role-expanded scopes combine? -> A: Union. Both sources combined into a single effective scope set.
- Q: What HTTP status for scope denial? -> A: 403 Forbidden with the required scope in the error message. Distinct from ownership 404 (FR-010 in Spec 040).
- Q: Should role hierarchy be implicit or explicit? -> A: Reference-based. Roles can reference other roles (e.g., `user: [viewer, responses:create, ...]`). Expanded at startup.
- Q: Should endpoint-to-scope mapping be hardcoded or configurable? -> A: Hardcoded. Endpoints are fixed, so their required scopes are too. No risk of misconfiguration.
- Q: How should permissions appear in API responses? -> A: Raw compact string (`"permissions": "rwd|r--|---"`).
- Q: Which resources support settable permissions? -> A: Vector stores and files. Responses and conversations stay private. Files and vector stores are tightly coupled in the RAG pipeline; sharing one without the other creates broken citations.

## User Scenarios & Testing

### User Story 1 - Endpoint Authorization via Scopes (Priority: P1)

An operator configures role-to-scope mappings for their antwort instance. Users are assigned roles (viewer, user, manager, admin) through their JWT claims. When a user calls an API endpoint, the system checks that the user's effective scopes (from JWT + role expansion) include the required scope for that endpoint. Users without the required scope receive a 403 Forbidden error with a clear message identifying the missing scope.

**Why this priority**: Scope-based authorization is the foundation that all other P2 features build on. Without it, any authenticated user can call any endpoint.

**Independent Test**: Configure role-to-scope mapping with a "viewer" role. Authenticate as a viewer. Call `POST /v1/responses`. Verify 403 with "insufficient scope: requires responses:create". Call `GET /v1/responses`. Verify success.

**Acceptance Scenarios**:

1. **Given** role-to-scope mapping is configured with viewer = `[responses:read]`, **When** a viewer calls `POST /v1/responses`, **Then** the system returns 403 Forbidden with message "insufficient scope: requires responses:create"
2. **Given** role-to-scope mapping is configured with viewer = `[responses:read]`, **When** a viewer calls `GET /v1/responses`, **Then** the request proceeds normally
3. **Given** a user has role "user" which references "viewer", **When** the user calls `GET /v1/responses`, **Then** the request succeeds (inherited scope from viewer)
4. **Given** a user has role "user" which includes `responses:create`, **When** the user calls `POST /v1/responses`, **Then** the request succeeds
5. **Given** a user has role "admin" mapped to `["*"]`, **When** the admin calls any endpoint, **Then** the request succeeds (wildcard matches all scopes)
6. **Given** a user's JWT contains explicit scopes `["responses:create"]` and their role expands to `["responses:read"]`, **When** the user calls `POST /v1/responses`, **Then** the request succeeds (union of both scope sources)
7. **Given** no role-to-scope mapping is configured, **When** any authenticated user calls any endpoint, **Then** the request proceeds normally (scope middleware is disabled for backward compatibility)

---

### User Story 2 - Shared Vector Stores (Priority: P1)

Alice creates a vector store and sets its permissions to allow group-read (`rwd|r--|---`). Bob, who shares the same tenant_id as Alice, can now see the vector store in his list results and use it for file_search. Dave, who has a different tenant_id, cannot see or search the vector store. The permissions are visible in API responses as a compact string.

**Why this priority**: Shared knowledge bases are the primary use case for multi-user RAG deployments. Without sharing, each user must duplicate their vector stores.

**Independent Test**: As Alice, create a vector store with `permissions: {"group": "r"}`. As Bob (same tenant), list vector stores. Alice's shared store appears. As Dave (different tenant), list vector stores. Alice's store does not appear.

**Acceptance Scenarios**:

1. **Given** Alice creates a vector store with default permissions, **When** Bob lists vector stores, **Then** Alice's store does not appear (default is private)
2. **Given** Alice creates a vector store with `permissions: {"group": "r"}`, **When** Bob (same tenant_id) lists vector stores, **Then** Alice's store appears in the list
3. **Given** Alice creates a vector store with `permissions: {"group": "r"}`, **When** Dave (different tenant_id) lists vector stores, **Then** Alice's store does not appear
4. **Given** Alice creates a vector store with `permissions: {"group": "r", "others": "r"}`, **When** Dave (different tenant_id) lists vector stores, **Then** Alice's store appears (others-readable)
5. **Given** a shared vector store exists, **When** Bob retrieves it by ID, **Then** the response includes `"permissions": "rwd|r--|---"`
6. **Given** a shared vector store has group-read permissions, **When** Bob uses it in `file_search` via `vector_store_ids`, **Then** search results from that store are returned to Bob
7. **Given** Bob references a vector store he cannot access in `vector_store_ids`, **Then** the store is silently skipped during file_search (no error, just no results from that store)
8. **Given** Alice created a vector store with `permissions: {"group": "r"}`, **When** Alice updates it with `permissions: {"group": "---"}`, **Then** Bob can no longer see the store in list results or use it for file_search

---

### User Story 3 - Shared Files (Priority: P2)

Alice uploads a file and sets its permissions to allow group-read. Bob (same tenant) can retrieve the file and sees citations referencing it. This prevents broken citations when a shared vector store indexes files that the searching user can't access.

**Why this priority**: Files and vector stores are tightly coupled in the RAG pipeline. Sharing vector stores without sharing files creates broken citation links.

**Independent Test**: As Alice, upload a file with `permissions: {"group": "r"}`. As Bob (same tenant), retrieve the file by ID. Verify success. Verify file_citation annotations referencing this file work for Bob.

**Acceptance Scenarios**:

1. **Given** Alice uploads a file with default permissions, **When** Bob requests it by ID, **Then** the system returns 404 (default is private)
2. **Given** Alice uploads a file with `permissions: {"group": "r"}`, **When** Bob (same tenant_id) requests it by ID, **Then** the file is returned
3. **Given** Alice uploads a file with `permissions: {"group": "r"}`, **When** Dave (different tenant_id) requests it by ID, **Then** the system returns 404
4. **Given** a shared file is indexed in a shared vector store, **When** Bob's file_search returns a result from that file, **Then** the `file_citation` annotation references a file Bob can read
5. **Given** Alice sets file permissions to `{"group": "r"}`, **When** Alice retrieves the file, **Then** `"permissions": "rwd|r--|---"` appears in the response

---

### User Story 4 - Vector Store Union Merge with Agent Profiles (Priority: P2)

An operator configures an agent profile "company-assistant" with default `vector_store_ids` pointing to shared company knowledge bases. When a user sends a request using this profile, the file_search tool searches both the profile's vector stores and any additional stores the user specifies. The merge rule is union (combine, deduplicate).

**Why this priority**: Enables operator-managed shared knowledge without requiring users to know store IDs. Builds on shared vector store permissions.

**Independent Test**: Configure agent profile with `vector_store_ids: ["vs_company"]`. Send request with `prompt: "company-assistant"` and additional `vector_store_ids: ["vs_my_notes"]`. Verify file_search uses both stores.

**Acceptance Scenarios**:

1. **Given** agent profile "assistant" has `vector_store_ids: ["vs_company"]`, **When** a user sends a request with `prompt: "assistant"` and `vector_store_ids: ["vs_mine"]`, **Then** file_search uses `["vs_company", "vs_mine"]`
2. **Given** agent profile has `vector_store_ids: ["vs_company"]`, **When** a user sends a request with `prompt: "assistant"` and no additional `vector_store_ids`, **Then** file_search uses `["vs_company"]`
3. **Given** both profile and request specify the same store ID, **When** file_search runs, **Then** the store is searched once (deduplicated)
4. **Given** the merged list includes a store the user cannot access, **When** file_search runs, **Then** the inaccessible store is silently skipped

---

### User Story 5 - Backward Compatibility (Priority: P1)

An operator upgrades to this version without configuring role-to-scope mapping or permissions. All existing behavior is preserved. No endpoints are blocked. No permissions are enforced. The scope middleware is a no-op when unconfigured.

**Why this priority**: Breaking existing deployments blocks adoption.

**Independent Test**: Start antwort with no authorization config. Call all endpoints. Verify all succeed as before.

**Acceptance Scenarios**:

1. **Given** no `role_scopes` is configured, **When** any authenticated user calls any endpoint, **Then** the request proceeds (no scope enforcement)
2. **Given** no `permissions` field is set on any resource, **When** resources are created and queried, **Then** default private permissions apply (`rwd|---|---`)
3. **Given** existing vector stores have no `permissions` column, **When** the migration runs, **Then** existing stores get default `rwd|---|---` and remain accessible to their owners

---

### Edge Cases

- What happens when a role references a non-existent role? The system rejects the configuration at startup with a clear error message identifying the undefined reference.
- What happens when role references form a cycle (A references B, B references A)? The system detects the cycle at startup and rejects the configuration.
- What happens when an owner tries to set permissions they don't have (e.g., granting "write" to group)? Allowed. The owner always has full control over their resource's permissions.
- What happens when a resource with group-read is deleted by the owner? The resource is soft-deleted and no longer visible to group members.
- What happens when the admin deletes a shared vector store? Admin can delete any resource in their tenant (per Spec 040). The shared store disappears for all users.
- What happens when a user has both JWT scopes and role scopes that partially overlap? Union merge. Overlapping scopes are deduplicated. The user gets the broadest access from either source.
- What happens when permissions are set to `---|---|---`? The owner level is always `rwd` (immutable). The system ignores attempts to set the owner level to anything other than `rwd`. Only group and others levels are settable. This is consistent with Spec 040 where the owner always has full access.
- What happens when Alice shares a vector store and later revokes sharing? Alice updates the store with `permissions: {"group": "---"}`. Bob can no longer see or search the store.

## Requirements

### Functional Requirements

- **FR-001**: System MUST enforce per-endpoint scope requirements when role-to-scope mapping is configured
- **FR-002**: System MUST compute effective scopes as the union of JWT-provided scopes and role-expanded scopes
- **FR-003**: System MUST support reference-based role hierarchy where roles can include other roles by name
- **FR-004**: System MUST resolve role references at startup and reject circular references with a clear error
- **FR-005**: System MUST return 403 Forbidden (not 401 or 404) when a user lacks the required scope, including the required scope name in the error message
- **FR-006**: System MUST skip scope enforcement entirely when no role-to-scope mapping is configured (backward compatibility)
- **FR-007**: System MUST treat the wildcard scope `*` as matching any required scope
- **FR-008**: System MUST store a `permissions` field (compact string format `owner|group|others`) on vector stores and files
- **FR-009**: System MUST default permissions to `rwd|---|---` (private) when not specified on creation
- **FR-010**: System MUST allow the resource owner to set group and others permissions on create and update via a `permissions` JSON object (e.g., `{"group": "r", "others": "r"}`). The owner level is always `rwd` and cannot be changed.
- **FR-018**: System MUST enforce scope requirements on file endpoints: `POST /v1/files` requires `files:create`, `GET /v1/files` and `GET /v1/files/{id}` require `files:read`, `DELETE /v1/files/{id}` requires `files:delete`
- **FR-011**: System MUST enforce permissions on list, get, and delete operations: group members can access resources where group permissions allow, others can access where others permissions allow
- **FR-012**: System MUST include the `permissions` compact string in API responses for vector stores and files
- **FR-013**: System MUST NOT allow setting permissions on responses or conversations (always private)
- **FR-014**: System MUST merge agent profile `vector_store_ids` with request `vector_store_ids` using union (combine, deduplicate) when both are present
- **FR-015**: System MUST silently skip vector stores in the merged list that the user cannot access (no error, no results from that store)
- **FR-016**: System MUST apply permission checks at file_search time: group-readable stores return results to group members
- **FR-017**: System MUST handle database migration for existing resources: add permissions column with default `rwd|---|---`

### Key Entities

- **Scope**: A string identifying an authorization capability (e.g., `responses:create`, `files:read`). Scopes are required per endpoint and granted via JWT claims or role expansion.
- **Role**: A named bundle of scopes (e.g., "viewer", "user", "manager", "admin"). Configured by the operator. Roles can reference other roles for inheritance.
- **Permissions**: A compact three-level string (`owner|group|others`) stored on shareable resources. Each level contains a combination of `r` (read), `w` (write), `d` (delete), or `-` (denied).

## Success Criteria

### Measurable Outcomes

- **SC-001**: When role-to-scope mapping is configured, users without the required scope receive 403 for every protected endpoint (100% enforcement verified by tests)
- **SC-002**: When role-to-scope mapping is not configured, all existing conformance and SDK tests pass without modification (backward compatibility)
- **SC-003**: Users in the same tenant can search shared vector stores and access shared files (verified by multi-user integration tests)
- **SC-004**: Users in different tenants cannot access resources shared only at the group level (tenant isolation maintained)
- **SC-005**: Agent profile vector store defaults are combined with request-specified stores (union merge verified by tests)
- **SC-006**: Invalid role configurations (undefined references, cycles) are rejected at startup with clear error messages

## Assumptions

- Spec 040 (Resource Ownership) is implemented. Owner field exists on all resources. Admin role extraction from JWT is working.
- The `Identity.Scopes` field is already populated by the JWT authenticator (Spec 007).
- The `Identity.Metadata["roles"]` field is populated by the JWT authenticator with roles from the configurable claim path (Spec 040).
- Only vector stores and files support settable permissions. Responses and conversations are always private. This can be extended in a future spec if needed.
- The scope middleware is an optional component. When not configured, it has no effect (nil-safe composition per constitution Principle III).

## Dependencies

- **Spec 040 (Resource Ownership)**: Owner field, admin role extraction, `storage.SetOwner/GetOwner/SetAdmin/GetAdmin` context helpers
- **Spec 007 (Auth)**: `Identity` struct, `Identity.Scopes`, JWT claim extraction, auth middleware
- **Spec 038 (Agent Profiles)**: Profile resolution, `vector_store_ids` in profile config, merge logic
- **Spec 034 (Files API)**: File metadata, `UserID`-based isolation (to be extended with permissions)
- **Brainstorm 33**: All P2 design decisions (scope merge, role hierarchy, permissions format, shareable types, endpoint mapping)
