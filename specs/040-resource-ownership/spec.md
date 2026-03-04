# Feature Specification: Resource Ownership

**Feature Branch**: `040-resource-ownership`
**Created**: 2026-03-04
**Status**: Draft
**Input**: User description: "Resource ownership: per-user data isolation with owner field on all resources, owner-based query filtering, and admin role override for multi-user deployments"

## Clarifications

### Session 2026-03-04

- Q: What is the terminology for multi-user vs multi-tenant? -> A: Per the constitution, multi-user means per-user data isolation within an instance. Multi-tenant means in-process isolation between user groups (tenants) via `tenant_id`. Multi-instance means separate Deployments for hard isolation. These compose: a single instance can be both multi-user and multi-tenant. This spec addresses per-user ownership and admin override (scoped to the admin's tenant).
- Q: What fields exist on resources today? -> A: Responses have `tenant_id` in storage but no owner/subject. Files have `UserID`. Conversations have `tenant_id`. Vector stores have no ownership fields. Scopes are extracted from JWT but never enforced.
- Q: Should this spec include Unix-style group/others permissions? -> A: No. The three-level permission model (`owner|group|others`) is deferred to a follow-up spec. This spec focuses on per-user ownership and admin override only. The permission columns will enable group sharing (within tenant) and others sharing (cross-tenant within instance) later.
- Q: What about backward compatibility? -> A: Single-user deployments (no auth configured) must continue to work unchanged. When no identity is present, ownership checks are skipped.
- Q: Should ownership denials produce an observable signal? -> A: Log at debug level via slog (subject, resource ID, action). No new metrics. Keeps 404 indistinguishable at the API level while giving operators visibility when debug logging is enabled. Consistent with Spec 026 (debug logging).

## User Scenarios & Testing

### User Story 1 - User Data Isolation (Priority: P1)

Alice and Bob are both users of the same antwort instance, authenticated via JWT. Alice creates responses, uploads files, and starts conversations. Bob does the same. Neither user can see, retrieve, or delete the other's data. Each user's API calls return only their own resources.

**Why this priority**: Data isolation is the core value proposition. Without it, multi-user deployments expose every user's data to every other user in the same user group.

**Independent Test**: Authenticate as Alice, create a response. Authenticate as Bob, list responses. Verify Alice's response does not appear in Bob's list. Verify Bob cannot retrieve Alice's response by ID.

**Acceptance Scenarios**:

1. **Given** Alice is authenticated and creates a response, **When** Bob lists responses via `GET /v1/responses`, **Then** Alice's response does not appear in the results
2. **Given** Alice is authenticated and creates a response, **When** Bob requests it by ID via `GET /v1/responses/{id}`, **Then** the system returns 404
3. **Given** Alice is authenticated and creates a conversation, **When** Bob lists conversations via `GET /v1/conversations`, **Then** Alice's conversation does not appear
4. **Given** Alice is authenticated and creates a vector store, **When** Bob lists vector stores via `GET /v1/vector_stores`, **Then** Alice's vector store does not appear
5. **Given** Alice is authenticated and uploads a file, **When** Bob lists files, **Then** Alice's file does not appear

---

### User Story 2 - Admin Read and Delete Override (Priority: P1)

Carol is an admin (determined by a JWT role claim). She can read and delete any user's resources within her tenant for administrative purposes (troubleshooting, compliance, cleanup). She cannot modify other users' resources, and she cannot access resources belonging to users in a different tenant.

**Why this priority**: Admins need visibility and cleanup capabilities. Without admin override, there is no way to manage orphaned resources, investigate issues, or enforce data policies.

**Independent Test**: Authenticate as Carol (admin role), list all responses. Verify Alice's and Bob's responses appear. Verify Carol can delete Alice's response. Verify Carol cannot modify Alice's response content.

**Acceptance Scenarios**:

1. **Given** Carol has the admin role and Alice has created a response, **When** Carol lists responses via `GET /v1/responses`, **Then** Alice's response appears in the results
2. **Given** Carol has the admin role, **When** Carol deletes Alice's response via `DELETE /v1/responses/{id}`, **Then** the response is deleted successfully
3. **Given** Carol has the admin role, **When** Carol lists conversations, **Then** all users' conversations in the instance are visible
4. **Given** Carol has the admin role, **When** Carol lists vector stores, **Then** all users' vector stores in the instance are visible
5. **Given** Bob does not have the admin role, **When** Bob attempts to list all resources, **Then** only Bob's own resources appear (admin override does not apply)
6. **Given** Carol has the admin role, **When** Carol attempts to add an item to Alice's conversation via `POST /v1/conversations/{id}/items`, **Then** the system returns 404 (admin cannot modify other users' resources)
7. **Given** Carol has the admin role with tenant_id "team-a", and Dave has created a response with tenant_id "team-b", **When** Carol lists responses, **Then** Dave's response does not appear (admin override is scoped to the admin's own user group)

---

### User Story 3 - Backward Compatibility (Priority: P1)

A developer runs antwort without authentication configured (NoOp authenticator, single-user mode). All resources are accessible without ownership checks. Upgrading to this version does not break existing single-user deployments.

**Why this priority**: Breaking backward compatibility would block adoption. The majority of quickstarts and development setups run without auth.

**Independent Test**: Start antwort with no auth config, create and retrieve responses. Verify all operations work as before. No 403/404 errors from ownership checks.

**Acceptance Scenarios**:

1. **Given** antwort runs with NoOp authenticator (no auth), **When** a user creates and lists responses, **Then** all responses are visible (no owner filtering)
2. **Given** antwort runs with API key auth but no tenant_id configured, **When** a user creates and lists responses, **Then** all responses created by that subject are visible
3. **Given** an existing deployment upgrades to this version with existing data in PostgreSQL, **When** the migration runs, **Then** existing resources remain accessible (empty owner matches all queries when no identity is present)

---

### User Story 4 - Owner Identity from Authentication (Priority: P2)

The owner of a resource is automatically set from the authenticated user's identity (`Identity.Subject`) at creation time. No API parameter is needed. The owner cannot be changed after creation.

**Why this priority**: Automatic ownership assignment is essential for the model to work, but it builds on the isolation foundation (P1).

**Independent Test**: Authenticate as Alice, create a response. Verify the stored response has Alice's subject as owner. Verify the owner cannot be set or overridden via API parameters.

**Acceptance Scenarios**:

1. **Given** Alice is authenticated with subject "alice", **When** she creates a response, **Then** the response's owner is set to "alice"
2. **Given** Alice is authenticated, **When** she creates a response and includes a `user` field in the request, **Then** the owner is still set from `Identity.Subject`, not from the request body
3. **Given** a response exists with owner "alice", **When** any update operation occurs, **Then** the owner field cannot be changed

---

### User Story 5 - Consistent 404 for Unauthorized Access (Priority: P2)

When a user tries to access a resource they don't own (and they don't have admin role), the system returns 404, not 403. This prevents leaking the existence of resources belonging to other users.

**Why this priority**: Security hardening. Returning 403 reveals that the resource exists, which is an information leak.

**Independent Test**: Authenticate as Bob, attempt to GET a response ID that belongs to Alice. Verify the response is 404, indistinguishable from a genuinely non-existent resource.

**Acceptance Scenarios**:

1. **Given** Alice owns response "resp_123", **When** Bob requests `GET /v1/responses/resp_123`, **Then** the system returns 404 (not 403)
2. **Given** response "resp_999" does not exist, **When** Bob requests `GET /v1/responses/resp_999`, **Then** the system returns 404 (identical response format to the case above)
3. **Given** Alice owns a conversation, **When** Bob requests it by ID, **Then** the system returns 404
4. **Given** Alice owns a vector store, **When** Bob requests `DELETE /v1/vector_stores/{id}`, **Then** the system returns 404

---

### Edge Cases

- What happens when a user's JWT has no `sub` claim? The auth middleware already rejects identities with empty subjects (existing behavior, no change needed).
- What happens when the admin role claim path is misconfigured? Admin override is not applied; the user is treated as a regular user. Configurable claim path has a sensible default.
- What happens when a response references `previous_response_id` owned by a different user? The chaining fails with 404, same as if the previous response didn't exist.
- What happens during the agentic loop when intermediate responses are stored? All intermediate responses inherit the owner from the request's authenticated identity.
- What happens when the `user` field in the OpenAI API request body differs from `Identity.Subject`? The `user` field is informational (for OpenAI compatibility). Ownership always comes from `Identity.Subject`.
- What happens when an API key user needs admin privileges? The API key's static identity can include the admin role in its configured metadata. The admin role check reads from `Identity`, not directly from JWT claims, so both auth methods support admin designation.

## Requirements

### Functional Requirements

- **FR-001**: System MUST store the owner (`Identity.Subject`) on every response, conversation, and vector store at creation time
- **FR-002**: System MUST filter all list queries (GET collection endpoints) by the authenticated user's subject, returning only resources owned by that user
- **FR-003**: System MUST filter all get-by-ID queries by the authenticated user's subject, returning 404 if the resource exists but is owned by a different user
- **FR-004**: System MUST filter all delete operations by the authenticated user's subject, returning 404 if the resource is owned by a different user
- **FR-005**: System MUST support an admin role (configurable role name, default "admin") that overrides ownership filtering for read and delete operations within the admin's own user group (`tenant_id`). An admin in one user group cannot access resources belonging to a different user group.
- **FR-006**: System MUST extract the admin role from JWT claims at a configurable claim path (default: `realm_access.roles`)
- **FR-007**: System MUST NOT allow admin users to modify (write/update) resources owned by other users
- **FR-008**: System MUST skip ownership checks when no identity is present in the request context (backward compatibility for NoOp authenticator)
- **FR-009**: System MUST set the owner from `Identity.Subject`, never from API request parameters
- **FR-010**: System MUST return 404 (not 403) when a user attempts to access a resource they do not own
- **FR-014**: System MUST log ownership denials at debug level including the requesting subject, resource ID, and attempted operation. No metrics or info-level logging for ownership checks.
- **FR-011**: System MUST NOT change file-related ownership filtering. The Files API already implements per-user isolation via `UserID` (set from authenticated identity, list/get filtered by owner, 404 for non-owner access). This spec aligns responses, conversations, and vector stores to the same pattern.
- **FR-012**: System MUST apply ownership filtering to `GET /v1/responses/{id}/input_items` based on the parent response's owner
- **FR-013**: System MUST handle database migration for existing data: resources without an owner field are accessible to all authenticated users (empty owner matches any subject)

### Key Entities

- **Owner**: The `Identity.Subject` of the user who created a resource. Stored as a string field on every resource record. Immutable after creation.
- **Admin Role**: A role name extracted from JWT claims (or configured on API key identities) that grants read and delete access to all resources within the admin's own tenant. The role name and claim path are configurable.
- **Resource**: Any API object with ownership: responses, conversations, vector stores, files. Files already have a `UserID` field that serves the same purpose.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In a multi-user deployment with two or more users, no user can retrieve, list, or delete another user's resources (100% isolation verified by acceptance tests)
- **SC-002**: Admin users can list and view all resources across all users in their tenant within the same response time as regular queries
- **SC-003**: Single-user deployments (NoOp authenticator) continue to pass all existing conformance and SDK tests without modification
- **SC-004**: Existing PostgreSQL databases upgrade without data loss; the migration adds the owner column with a default that preserves existing access patterns
- **SC-005**: Unauthorized access attempts return 404 responses indistinguishable from requests for genuinely non-existent resources

## Assumptions

- The Files API already has `UserID`-based isolation via `userFromCtx`. This spec aligns the other resource types (responses, conversations, vector stores) to the same pattern.
- The `Identity.Subject` field is always non-empty for authenticated requests (enforced by existing auth middleware).
- Group and others sharing (three-level permissions with `owner|group|others` format) is deferred to a follow-up spec. This spec only adds ownership isolation and admin override. The follow-up spec will enable sharing resources within a tenant (group) or across tenants within an instance (others).
- Scope-based endpoint authorization (can this user call POST /v1/files at all?) is deferred to a follow-up spec. This spec does not add scope middleware.
- The admin role is a binary check (has role or doesn't). Role hierarchy and role-to-scope expansion are deferred.

## Dependencies

- **Spec 007 (Auth)**: Provides `Identity` struct, auth middleware, JWT claim extraction, and `Identity.Subject`
- **Spec 005 (Storage)**: Provides `ResponseStore` interface and tenant-scoped queries
- **Spec 037 (Conversations API)**: Provides `ConversationStore` interface
- **Spec 039 (Vector Store Unification)**: Provides unified vector store backend interface
- **Spec 034 (Files API)**: Already implements `UserID`-based isolation (reference pattern)
- **Brainstorm 33**: Design decisions for the authorization model, including P1/P2 phasing
