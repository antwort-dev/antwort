# Research: Resource Ownership

## Decision 1: Owner Storage Pattern

**Decision**: Add `owner` column alongside existing `tenant_id` column. Follow the same context-based injection pattern.

**Rationale**: The codebase already uses `storage.GetTenant(ctx)` for tenant scoping. Adding a parallel `storage.GetOwner(ctx)` (or extracting from `auth.IdentityFromContext(ctx).Subject`) keeps the pattern consistent. The Files API proves this approach works via `UserID` + `userFromCtx`.

**Alternatives considered**:
- Reuse `tenant_id` for ownership: Rejected because tenant and owner serve different purposes (group vs individual isolation)
- Store owner in `Identity.Metadata`: Rejected because owner is a first-class storage concern, not metadata

## Decision 2: Admin Role Extraction

**Decision**: Add `RolesClaim` config field to JWT authenticator. Extract roles from JWT at auth time, store in `Identity.Metadata["roles"]` as comma-separated string.

**Rationale**: Keycloak puts roles in `realm_access.roles` (nested JSON). Other OIDC providers use different paths. A configurable claim path handles all providers. Storing in metadata avoids changing the Identity struct (backward compatible).

**Alternatives considered**:
- Add dedicated `Roles []string` field to Identity: Cleaner but breaks the existing struct. Deferred to a broader auth refactor.
- Check JWT claims at authorization time (re-parse token): Rejected because it duplicates work the auth middleware already does.

## Decision 3: Authorization Check Location

**Decision**: Ownership filtering happens in the storage layer (same as tenant filtering), not in a middleware or handler.

**Rationale**: The codebase already filters by tenant at the storage query level. Adding owner filtering at the same level keeps the pattern consistent. Every store method already receives context, so adding owner checks is mechanical. This prevents handlers from forgetting to check ownership.

**Alternatives considered**:
- Authorization middleware: Would require a generic way to identify the resource being accessed before the handler runs. Too complex for the current architecture.
- Handler-level checks: Would require every handler to remember to call `CanAccess()`. Error-prone and duplicative.

## Decision 4: Migration Strategy for Existing Data

**Decision**: Add `owner` column with default empty string (`''`). In query logic, empty owner matches any authenticated user.

**Rationale**: Existing deployments have responses, conversations, and vector stores without owners. Setting owner to empty string means these resources remain accessible to all users (no breaking change). New resources get the owner set from `Identity.Subject`. Over time, old data can be migrated or cleaned up by admins.

**Alternatives considered**:
- Require a migration script that assigns owners: Rejected because there's no way to determine the correct owner for existing data.
- Set owner to a sentinel value like `__system__`: Rejected because it introduces a magic value. Empty string is simpler and consistent with the existing empty `tenant_id` pattern.

## Decision 5: Vector Store Ownership

**Decision**: Vector stores get an owner field following the same pattern as responses and conversations. The vector store backend interface does not change. Ownership is on the management layer (create/list/get/delete vector stores), not on individual vectors.

**Rationale**: The vector store API endpoints handle create/list/get/delete. These are where ownership filtering applies. The actual vector search (embedding lookup) happens through the engine during file_search tool execution, which already has the user's context.

**Alternatives considered**:
- Per-document ownership within a vector store: Rejected for P1. Too complex, would require post-filtering search results. Deferred to follow-up spec (three-level permissions).
