# Brainstorm 33: Fine-Grained Authorization

**Date**: 2026-03-03 (updated 2026-03-04)
**Participants**: Roland Huss
**Goal**: Design a three-layer authorization model: endpoint-level scopes, CRUD-level roles, and resource-level ownership with Unix-style permissions.

## Terminology (from Constitution)

The constitution defines three isolation levels that compose:

- **Multi-user**: A single antwort instance serving multiple individual users. Data isolated by ownership (`Identity.Subject`).
- **Multi-tenant**: A single antwort instance serving multiple user groups (tenants). Data isolated in-process by `tenant_id`. Users in one tenant cannot see resources from another tenant.
- **Multi-instance**: Multiple antwort Deployments for hard isolation (different organizations, compliance boundaries). Isolation at infrastructure level.

In authorization terms:
- **Owner** = `Identity.Subject` (the individual user, e.g., "alice")
- **Group** = `Identity.Metadata["tenant_id"]` (the tenant within this instance, e.g., "team-backend")
- **Others** = users in different tenants within the same instance

## Core Principle

**Data ownership is non-negotiable.** When a user uploads a file, creates a conversation, or generates a response, that data belongs to them. No role, scope, or admin override can bypass ownership unless explicitly granted. This is the foundation, not a feature.

## Current State

Antwort has authentication (JWT/OIDC, API keys) and user-group scoping (storage filtered by `tenant_id` from JWT claims). But within a user group, every user can see every other user's data. There is no per-user ownership enforcement.

```
Today:    Authn ✅ → User-group scoping ✅ → User isolation ❌ → Authorization ❌
Target:   Authn ✅ → User-group scoping ✅ → User isolation ✅ → Authorization ✅
```

## Three-Layer Design

```
Level 1: Endpoint Access (scopes)     → Can you call POST /v1/files at all?
Level 2: CRUD Authorization (roles)   → Roles bundle scopes for common personas
Level 3: Resource Ownership (per-obj) → Can you access THIS specific file?
```

All three layers are enforced independently. Passing Level 1 does not bypass Level 3.

### Level 1: Endpoint-Level Scopes

Every endpoint requires a scope. Scopes are fine-grained, per resource type and operation:

| Resource | Scopes |
|----------|--------|
| Responses | `responses:create`, `responses:read`, `responses:delete` |
| Files | `files:create`, `files:read`, `files:delete` |
| Vector Stores | `vector_stores:create`, `vector_stores:read`, `vector_stores:write`, `vector_stores:delete` |
| Conversations | `conversations:create`, `conversations:read`, `conversations:write`, `conversations:delete` |
| Agents | `agents:read` |

Scopes come from the JWT `scope` claim (space-separated or array). A scope middleware checks the required scope for each endpoint before the handler runs.

### Level 2: Role-Based Scope Bundles (Keycloak)

Roles are managed in Keycloak and bundle scopes for common personas:

| Role | Scopes Included | Use Case |
|------|----------------|----------|
| **viewer** | `*:read`, `agents:read` | Read-only dashboards, monitoring |
| **user** | viewer + `responses:create`, `files:create`, `conversations:create`, `conversations:write` | Standard chat user |
| **manager** | user + `vector_stores:*`, `files:delete`, `conversations:delete`, `responses:delete` | RAG infrastructure management |
| **admin** | All scopes + admin resource override | Full tenant administration |

Role-to-scope expansion happens in antwort at request time. Keycloak assigns roles, antwort knows the mapping.

### Level 3: Resource-Level Ownership (Always Enforced)

### Layer 1: User-Level Ownership (Always Enforced)

Every resource stores its owner's `Subject` (from `Identity.Subject`). All queries filter by owner by default:

```
GET /v1/files        → returns only files where owner == current user
GET /v1/conversations → returns only conversations where owner == current user
```

This is not a permission check. It is the storage query. There is no "show all files" endpoint for regular users. You see your own data, period.

### Layer 2: Three-Level Permissions (Per Resource)

Each resource has a permission triple:

```
owner|group|others
 rwd | rwd | rwd
```

Where:
- **owner**: The user who created the resource (always `rwd` for the creator)
- **group**: All users sharing the same `tenant_id` within this instance
- **others**: Users in different `tenant_id` values within the same instance

Permissions:
- **r** (read): Can view/download/search the resource
- **w** (write): Can modify/update the resource
- **d** (delete): Can delete the resource

**Defaults by resource type**:

| Resource | Default | Effect |
|----------|---------|--------|
| Files | `rwd\|---\|---` | Strictly private |
| Conversations | `rwd\|---\|---` | Strictly private |
| Responses | `rwd\|---\|---` | Strictly private |
| Vector Stores | `rwd\|---\|---` | Private (can be shared within or across tenants) |

**Changing permissions**: The owner can set group and others permissions when creating or updating a resource:

```json
POST /v1/vector_stores
{
  "name": "team-docs",
  "permissions": {"group": "r"}
}
```

This makes the vector store searchable by all users sharing the same `tenant_id`.

```json
POST /v1/vector_stores
{
  "name": "company-wide-policies",
  "permissions": {"group": "r", "others": "r"}
}
```

This makes the vector store searchable by all users in the instance, regardless of `tenant_id`.

### Layer 3: Role-Based Admin Access (Keycloak)

The `admin` role (from JWT claims) grants elevated access within the admin's own tenant:

- Admins can read all resources in their tenant (overrides `group: ---`)
- Admins can delete any resource in their tenant
- Admin access is determined by a JWT claim (e.g., `realm_access.roles` contains `"admin"`)

The admin role comes from JWT claims (e.g., Keycloak realm roles). Antwort reads the role and applies the override logic. The OIDC provider is not prescribed.

**Role hierarchy**:

| Role | Own data | Group data (shared) | All data in own tenant |
|------|----------|-------------------|------------------------|
| user (default) | rwd | per group permissions | No override |
| admin | rwd | rwd | read + delete |

### Implementation Sketch

**1. Resource metadata** (extends all resource types):

```
Owner:       string    // Identity.Subject of creator
TenantID:    string    // Tenant of creator
Permissions: string    // "rwd|---|---" (compact string, three levels: owner|group|others)
```

**2. Authorization check** (new middleware or helper):

```
func CanAccess(identity *Identity, resource Resource, operation Op) bool {
    // Owner always has full access
    if resource.Owner == identity.Subject {
        return true
    }

    // Admin role: read + delete on resources in the admin's own tenant
    if hasRole(identity, "admin") && resource.TenantID == identity.TenantID() {
        return operation == Read || operation == Delete
    }

    // Group permissions: check if user shares the same tenant_id
    if resource.TenantID == identity.TenantID() {
        return resource.Permissions.Group.Has(operation)
    }

    // Others permissions: user is in a different tenant within the same instance
    return resource.Permissions.Others.Has(operation)
}
```

**3. Keycloak configuration**:

In Keycloak, the `admin` role is assigned via realm roles. The JWT includes:

```json
{
  "sub": "alice",
  "tenant_id": "org-1",
  "realm_access": {
    "roles": ["admin"]
  }
}
```

Antwort reads `realm_access.roles` (configurable claim path) to determine admin status.

## Enforcement Architecture

```
Request
  │
  ▼
Auth Middleware (existing)
  │ Extracts Identity: subject, tenant, scopes, roles
  │
  ▼
Scope Middleware (NEW - Level 1)
  │ Expands roles → scopes (role-scope mapping in config)
  │ Checks: does Identity have required scope for this endpoint?
  │ 404 if missing scope (don't leak endpoint existence)
  │
  ▼
Handler (Level 3 checks)
  │ Calls CanAccess(identity, resource, operation)
  │ Owner always has access
  │ Admin role overrides for tenant resources
  │ Group permissions for shared resources
  │ 404 if no access (don't leak resource existence)
  │
  ▼
Storage (existing tenant filter + NEW owner filter)
```

Key: Scopes control "can you call this endpoint." Ownership controls "can you see this data." Both enforced independently.

## What Changes

### Storage Layer

Every store (ResponseStore, FileMetadataStore, ConversationStore, VectorStoreFileStore) needs:
- **Owner field** on all records (already have `UserID` on files and conversations, missing on responses and vector stores)
- **Permissions field** on shareable resources (vector stores initially)
- **Query filtering** by owner (not just tenant)

### Auth/Authz Middleware

- Extract roles from JWT claims (new: `roles_claim` config)
- Add `Identity.Roles` field (or use Metadata)
- New scope middleware: expand roles to scopes, check required scope per endpoint
- Role-to-scope mapping in config (not hardcoded)

### API Changes

- `permissions` field on create/update for shareable resources
- 404 for unauthorized access (don't leak resource existence)

### Configuration

```yaml
auth:
  jwt:
    roles_claim: realm_access.roles   # Keycloak default path
  authorization:
    admin_role: admin
    role_scopes:
      viewer: [responses:read, files:read, vector_stores:read, conversations:read, agents:read]
      user: [viewer, responses:create, files:create, conversations:create, conversations:write]
      manager: [user, vector_stores:create, vector_stores:read, vector_stores:write, vector_stores:delete, files:delete, conversations:delete, responses:delete]
      admin: ["*"]
```

## OIDC Provider Reference Architecture (Keycloak Example)

```
Keycloak Realm: antwort
├── Clients
│   └── antwort-gateway
│       ├── Audience: antwort-gateway
│       └── Protocol Mappers:
│           ├── tenant_id → custom claim from user attribute
│           └── realm roles → realm_access.roles (default)
├── Realm Roles
│   ├── viewer
│   ├── user (default, assigned to all new users)
│   ├── manager
│   └── admin
├── Users
│   ├── alice (roles: user, tenant: org-1)
│   ├── bob (roles: user, tenant: org-1)
│   └── carol (roles: admin, tenant: org-1)
└── Groups (optional, maps to tenants)
    └── org-1
        ├── alice
        ├── bob
        └── carol
```

## Scope

### In Scope (spec candidate)
- Scope middleware: per-endpoint scope enforcement
- Role-to-scope mapping in config
- Owner field on all resources
- Owner-based query filtering (user sees only own data)
- Unix-style permissions on vector stores (group read for sharing)
- Admin role from JWT for elevated tenant access
- Keycloak configuration for roles claim

### Out of Scope
- Cross-instance sharing (blocked by separate Deployments, outside authorization scope)
- Per-resource ACLs (explicit user lists, not just owner/group/others)
- Permission inheritance (folder-like hierarchies)
- Audit logging (separate concern)
- OAuth2 scope-based endpoint authorization (deferred)
- Custom roles beyond user/admin

## Resolved Questions (2026-03-04 session)

### Q1: Permission storage format
**Decision: Compact string, three levels.** Store as `rwd|---|---`. Human-readable in DB queries, easy to log, negligible parse cost. Three levels match the three isolation boundaries: owner (user), group (tenant within instance), others (cross-tenant within instance).

```sql
-- PostgreSQL column
permissions TEXT NOT NULL DEFAULT 'rwd|---|---'
```

```go
type Permissions struct {
    Owner  PermSet  // parsed from segment 0
    Group  PermSet  // parsed from segment 1
    Others PermSet  // parsed from segment 2
}

func ParsePermissions(s string) Permissions
func (p Permissions) String() string
```

### Q2: Admin override configurability
**Decision: Fixed behavior.** Admin always gets read + delete on all resources in the instance. Cannot write/modify other users' resources. No config knob beyond the role name itself (`auth.authorization.admin_role: admin`).

### Q3: Shared vector stores and file_search
**Decision: Explicit in vector_store_ids + agent profile defaults (union merge).**

Agent profiles provide a base set of vector stores (operator-configured). User requests can add more via `vector_store_ids`. The merge rule is **union** (combine both lists, deduplicate). Permissions are enforced at search time: if a user references a store they can't access, it's skipped.

```
Profile:   [vs_company_docs, vs_policies]
Request:   [vs_my_notes]
Effective: [vs_company_docs, vs_policies, vs_my_notes]
```

No auto-discovery of group-readable stores. Users discover shared stores via `GET /v1/vector_stores` (returns both owned and group-readable stores).

### Q4: Agent profile scoping
**Decision: Instance-wide.** Profiles are server config, visible to all users of the Deployment. Per-user-group profile visibility is not needed because separate tenants get separate Deployments with their own config (per constitution's multi-tenant definition).

## Spec Phasing

### P1: Resource Ownership (spec candidate now)
- Owner (`Identity.Subject`) field on responses, conversations, vector stores
- Storage queries filter by owner (not just user-group/tenant_id)
- Admin role: read + delete all resources in instance
- 404 for unauthorized access (don't leak resource existence)
- Extract admin role from JWT claims (configurable claim path)
- Backward compatible: single-user deployments (no auth) continue to work unchanged

### P2: Scopes and Sharing (next spec)
- Scope middleware: per-endpoint scope enforcement
- Role-to-scope mapping config (viewer, user, manager, admin)
- Three-level permissions on vector stores (group sharing via `rwd|r--|---`, instance-wide via `rwd|r--|r--`)
- `permissions` field on create/update API for shareable resources
- Vector store union merge with agent profile defaults

## P2 Design Decisions (2026-03-04 session)

### Scope Middleware

- Sits after auth middleware (needs `Identity` in context). Chain: auth -> scope -> handler.
- Checks required scope for each endpoint against the user's effective scopes.
- **Effective scopes = JWT scopes (union) role-expanded scopes**. Both sources combined.
- **403 Forbidden** for scope denial (not 404). Includes the required scope in the error message.
- **Nil-safe**: when no scope config is present, middleware is a no-op (backward compat).
- **`*` wildcard** in admin role means all scopes. Middleware checks for `*` first.

### Role-to-Scope Mapping

- Reference-based hierarchy: roles can reference other roles.
- Expansion happens at startup (not per-request). Circular references are detected and rejected.
- Config:

```yaml
auth:
  authorization:
    admin_role: admin
    role_scopes:
      viewer: [responses:read, files:read, vector_stores:read, conversations:read, agents:read]
      user: [viewer, responses:create, files:create, conversations:create, conversations:write]
      manager: [user, vector_stores:create, vector_stores:write, vector_stores:delete, files:delete, conversations:delete, responses:delete]
      admin: ["*"]
```

### Endpoint-to-Scope Mapping

- **Hardcoded** in code (not configurable). Endpoints are fixed, so their required scopes are too.
- Complete mapping:

| Endpoint | Required Scope |
|----------|---------------|
| `POST /v1/responses` | `responses:create` |
| `GET /v1/responses` | `responses:read` |
| `GET /v1/responses/{id}` | `responses:read` |
| `GET /v1/responses/{id}/input_items` | `responses:read` |
| `DELETE /v1/responses/{id}` | `responses:delete` |
| `POST /v1/conversations` | `conversations:create` |
| `GET /v1/conversations` | `conversations:read` |
| `GET /v1/conversations/{id}` | `conversations:read` |
| `DELETE /v1/conversations/{id}` | `conversations:delete` |
| `GET /v1/conversations/{id}/items` | `conversations:read` |
| `POST /v1/conversations/{id}/items` | `conversations:write` |
| `GET /v1/vector_stores` | `vector_stores:read` |
| `GET /v1/vector_stores/{id}` | `vector_stores:read` |
| `POST /v1/vector_stores` | `vector_stores:create` |
| `DELETE /v1/vector_stores/{id}` | `vector_stores:delete` |
| `GET /v1/agents` | `agents:read` |

### Permissions in API Responses

- **Raw compact string** in API responses: `"permissions": "rwd|r--|---"`.
- Settable on create/update via `"permissions": {"group": "r"}` (JSON object for input, compact string in response).
- Only vector stores and files support settable permissions. Responses and conversations are always `rwd|---|---`.

### Shareable Resource Types

| Resource | Default Permissions | Changeable | Why |
|----------|-------------------|------------|-----|
| Vector Stores | `rwd\|---\|---` | Yes | Shared RAG knowledge bases |
| Files | `rwd\|---\|---` | Yes | Shared files avoid broken citations when vector store is shared |
| Responses | `rwd\|---\|---` | No | Private by nature |
| Conversations | `rwd\|---\|---` | No | Private by nature |

When a vector store has `group:r`, file_search results from that store are readable by group members. Individual file `GET` follows the file's own permissions.

### Vector Store Union Merge with Agent Profiles

- Agent profiles provide a base set of `vector_store_ids` (operator-configured).
- User requests can add more via `vector_store_ids` in the tool config.
- Merge rule: **union** (combine both lists, deduplicate).
- Permissions checked at search time. If user can't access a store (not owner, no group/others permission), it's silently skipped.
- Users discover shared stores via `GET /v1/vector_stores` (returns owned + group-readable + others-readable).

## Open Questions (remaining)

None. All design decisions for P2 are resolved. Ready for `/speckit.specify`.
