# Brainstorm 33: Fine-Grained Authorization with Keycloak

**Date**: 2026-03-03
**Participants**: Roland Huss
**Goal**: Design a three-layer authorization model: endpoint-level scopes, CRUD-level roles, and resource-level ownership with Unix-style permissions.

## Core Principle

**Data ownership is non-negotiable.** When a user uploads a file, creates a conversation, or generates a response, that data belongs to them. No role, scope, or admin override can bypass ownership unless explicitly granted. This is the foundation, not a feature.

## Current State

Antwort has authentication (JWT/OIDC via Keycloak, API keys) and tenant isolation (storage filtered by `tenant_id` from JWT claims). But within a tenant, every user can see every other user's data. There is no user-level ownership enforcement.

```
Today:    Authn ✅ → Tenant isolation ✅ → User isolation ❌ → Authorization ❌
Target:   Authn ✅ → Tenant isolation ✅ → User isolation ✅ → Authorization ✅
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

### Layer 2: Unix-Style Permissions (Per Resource)

Each resource has a permission triple:

```
owner|group|others
 rwd | rwd | rwd
```

Where:
- **owner**: The user who created the resource (always `rwd` for the creator)
- **group**: The tenant (organization) the owner belongs to
- **others**: Cross-tenant access (always `---` due to tenant isolation)

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
| Vector Stores | `rwd\|---\|---` | Private (can be shared) |

**Changing permissions**: The owner can set group permissions when creating or updating a resource:

```json
POST /v1/vector_stores
{
  "name": "company-docs",
  "permissions": {"group": "r"}
}
```

This makes the vector store searchable by all users in the tenant.

### Layer 3: Role-Based Admin Access (Keycloak)

The `admin` role in Keycloak grants elevated access within the tenant:

- Admins can read all resources in their tenant (overrides `group: ---`)
- Admins can delete any resource in their tenant
- Admin access is determined by a JWT claim (e.g., `realm_access.roles` contains `"admin"`)

The admin role is a Keycloak concept, not an antwort concept. Antwort reads the role from the JWT and applies the override logic.

**Role hierarchy**:

| Role | Own data | Group data (shared) | All tenant data |
|------|----------|-------------------|-----------------|
| user (default) | rwd | r (if group=r) | No access |
| admin | rwd | rwd | read + delete |

### Implementation Sketch

**1. Resource metadata** (extends all resource types):

```
Owner:       string    // Identity.Subject of creator
TenantID:    string    // Tenant of creator
Permissions: string    // "rwd|---|---" (compact string representation)
```

**2. Authorization check** (new middleware or helper):

```
func CanAccess(identity *Identity, resource Resource, operation Op) bool {
    // Owner always has full access
    if resource.Owner == identity.Subject {
        return true
    }

    // Admin role: read + delete on all tenant data
    if hasRole(identity, "admin") && resource.TenantID == identity.TenantID() {
        return operation == Read || operation == Delete
    }

    // Group permissions: check if user is in same tenant
    if resource.TenantID == identity.TenantID() {
        return resource.GroupPermissions.Has(operation)
    }

    return false
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

## Keycloak Reference Architecture

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
- Cross-tenant sharing (always blocked by tenant isolation)
- Per-resource ACLs (explicit user lists, not just owner/group/others)
- Permission inheritance (folder-like hierarchies)
- Audit logging (separate concern)
- OAuth2 scope-based endpoint authorization (deferred)
- Custom roles beyond user/admin

## Open Questions

1. Should the permissions string be stored as a compact string ("rwd|r--|---") or as a structured type?
2. Should admin override be configurable (e.g., admin_can_read_all: true/false)?
3. Should shared vector stores be searchable via file_search without explicitly listing them in vector_store_ids?
4. How does this interact with agent profiles? Should profiles be tenant-scoped or global?
