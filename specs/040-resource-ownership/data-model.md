# Data Model: Resource Ownership

## Entities

### Owner (context-derived, not stored separately)

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| subject | string | `Identity.Subject` | The unique user identifier from authentication |

Owner is not a standalone entity. It is extracted from the request context at creation time and stored as a field on each resource.

### Response (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | string | yes | `resp_` prefix + 24 alphanumeric chars |
| owner | string | yes | `Identity.Subject` of creator. Immutable. |
| tenant_id | string | yes | User group identifier. Empty string for single-tenant. |
| status | string | yes | pending, in_progress, completed, failed, cancelled, incomplete |
| model | string | yes | Model identifier |
| ... | ... | ... | Existing fields unchanged |

**New field**: `owner` (added alongside existing `tenant_id`)

### Conversation (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | string | yes | `conv_` prefix |
| owner | string | yes | `Identity.Subject` of creator. Immutable. |
| tenant_id | string | yes | User group identifier |
| name | string | no | User-provided name |
| ... | ... | ... | Existing fields unchanged |

**New field**: `owner` (added alongside existing `tenant_id`)

### Vector Store (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | string | yes | `vs_` prefix |
| owner | string | yes | `Identity.Subject` of creator. Immutable. |
| tenant_id | string | yes | User group identifier |
| name | string | no | User-provided name |
| ... | ... | ... | Existing fields unchanged |

**New field**: `owner` (added alongside existing `tenant_id`)

### File (unchanged)

Already has `UserID` field serving the same purpose as `owner`. No changes needed.

## Storage Schema Changes

### PostgreSQL Migration (new migration file)

```sql
-- Add owner column to responses
ALTER TABLE responses ADD COLUMN owner TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_responses_owner ON responses(owner);

-- Add owner column to conversations (if table exists)
ALTER TABLE conversations ADD COLUMN owner TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_conversations_owner ON conversations(owner);

-- Add owner column to vector_stores (if table exists)
ALTER TABLE vector_stores ADD COLUMN owner TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_vector_stores_owner ON vector_stores(owner);
```

Default empty string ensures backward compatibility: existing rows are accessible when no owner filtering is active.

### In-Memory Store Changes

Add `owner` field to entry structs:

```
entry struct:        +owner string (for responses)
convEntry struct:    +owner string (for conversations)
vectorStoreEntry:    +owner string (for vector stores)
```

## Query Filtering Rules

### Standard User (no admin role)

| Operation | Filter | Result if not owner |
|-----------|--------|---------------------|
| List | `WHERE owner = $subject AND tenant_id = $tenant` | Resource excluded from results |
| Get by ID | `WHERE id = $id AND owner = $subject AND tenant_id = $tenant` | 404 |
| Delete | `WHERE id = $id AND owner = $subject AND tenant_id = $tenant` | 404 |
| Create | Set `owner = $subject, tenant_id = $tenant` | N/A |

### Admin User (within own tenant)

| Operation | Filter | Result |
|-----------|--------|--------|
| List | `WHERE tenant_id = $tenant` (no owner filter) | All resources in tenant |
| Get by ID | `WHERE id = $id AND tenant_id = $tenant` (no owner filter) | Resource visible |
| Delete | `WHERE id = $id AND tenant_id = $tenant` (no owner filter) | Resource deleted |
| Write/Update | `WHERE id = $id AND owner = $subject AND tenant_id = $tenant` | 404 if not owner (admin cannot modify) |

### No Identity (NoOp authenticator)

| Operation | Filter | Result |
|-----------|--------|--------|
| All | No owner or tenant filter | All resources visible |

## Relationships

```
Identity (auth context)
  |
  |-- Subject --> Resource.owner (1:many)
  |-- TenantID --> Resource.tenant_id (1:many)
  |
Resource (response, conversation, vector store, file)
  |-- owner: string (immutable, set at creation)
  |-- tenant_id: string (from user group)
```

## State Transitions

Owner field has no state transitions. It is set once at creation and never changes. There is no API to transfer ownership. Admin deletion removes the resource entirely (soft delete), not ownership.
