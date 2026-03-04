# Research: Scope-Based Authorization and Resource Permissions

## Decision 1: Scope Middleware Architecture

**Decision**: New standalone scope middleware that wraps after auth middleware. Checks effective scopes (union of JWT + role-expanded) against hardcoded endpoint-to-scope map. Returns 403 when missing scope. No-op when unconfigured.

**Rationale**: The auth middleware already injects `Identity` with `Scopes` populated from JWT. A separate middleware keeps scope enforcement decoupled from authentication. The existing middleware stacking pattern in `cmd/server/main.go` makes this a simple addition.

**Alternatives considered**:
- Embed in auth middleware: Rejected because it conflates authentication (who are you?) with authorization (what can you do?). Harder to disable independently.
- Per-handler checks: Rejected because every handler would need to check scopes, creating duplication and risk of omission.

## Decision 2: Role Expansion at Startup

**Decision**: Resolve role references into flat scope lists at startup. Store the expanded map. Look up at request time is a simple map access, not a recursive resolution.

**Rationale**: Role hierarchies are config-driven and don't change at runtime. Resolving at startup means O(1) per-request lookup and catches circular references early with clear error messages.

**Alternatives considered**:
- Lazy per-request resolution with caching: Rejected because it defers error detection and adds complexity for no benefit (config doesn't change at runtime).

## Decision 3: Permissions Storage and Enforcement

**Decision**: Add `permissions` string field to `VectorStore` and `File` structs. Parse on read using a shared `Permissions` type with `Owner`, `Group`, `Others` fields. Enforce in the same locations where owner checks already happen (from Spec 040).

**Rationale**: The owner-check pattern (`ownerAllowed`, `vsOwnerAllowed`, `userFromCtx`) is already established. Permission checks extend this pattern: after owner check, check group and others permissions. The compact string format (`rwd|r--|---`) stores compactly and parses trivially.

**Alternatives considered**:
- Separate permission table with foreign keys: Rejected as over-engineering for three permission levels.
- Store as JSON object: Rejected in brainstorm 33. Compact string is simpler and human-readable in DB queries.

## Decision 4: Vector Store Union Merge Location

**Decision**: The merge happens in `pkg/agent/merge.go` alongside the existing tool union merge. Profile `VectorStoreIDs` are combined with request-specified `vector_store_ids` from the file_search tool config. Deduplicated by ID.

**Rationale**: The merge logic for tools already follows the union pattern in `merge.go`. Adding vector store ID merge in the same function keeps all profile merge logic in one place.

**Alternatives considered**:
- Merge at search time in the file_search provider: Rejected because the provider shouldn't know about agent profiles. Merge belongs in the profile resolution layer.

## Decision 5: Permission Checks in File Search

**Decision**: Permission checks happen in `pkg/tools/builtins/filesearch/provider.go` at the point where stores are resolved for search (lines 249-268). After tenant check and owner check, add permission check. Inaccessible stores are silently skipped.

**Rationale**: This is the only location where vector_store_ids are resolved to actual stores for searching. Adding permission checks here ensures they're enforced for all search paths (explicit IDs and tenant-wide search).

**Alternatives considered**:
- Check permissions in the HTTP handler before calling Execute: Rejected because the handler doesn't know which stores will be searched (that's determined inside Execute based on arguments).
