# Research: Async Responses (Background Mode)

**Feature**: 044-async-responses
**Date**: 2026-03-05

## R-001: Storage Update Capability

**Decision**: Extend `ResponseStore` interface with `UpdateResponse` method for status transitions.

**Rationale**: The current `ResponseStore` interface only supports `SaveResponse` (INSERT) and `DeleteResponse` (soft-delete). Background mode requires updating response status (`queued` -> `in_progress` -> `completed`/`failed`) and writing output/error after processing completes. Adding `UpdateResponse(ctx, id, updates)` is the minimal extension needed.

**Alternatives considered**:
- Full replace via `SaveResponse`: rejected because it creates race conditions (one caller could overwrite another's changes) and doesn't support atomic claim semantics.
- Separate `StatusStore` interface: rejected because it fragments the storage contract unnecessarily. A single `UpdateResponse` with a typed update struct keeps the interface cohesive.

## R-002: Atomic Claim Mechanism

**Decision**: Use `ClaimQueuedResponse(ctx, workerID)` as a dedicated method that atomically transitions a `queued` response to `in_progress` and assigns a worker ID.

**Rationale**: Multiple workers polling concurrently need a mechanism to claim exactly one request without races. In PostgreSQL, this maps to `UPDATE ... WHERE status = 'queued' ... LIMIT 1 RETURNING *` (or equivalent with `FOR UPDATE SKIP LOCKED`). In-memory storage uses a mutex-guarded scan. A dedicated method makes the atomic-claim contract explicit rather than relying on callers to build it from lower-level primitives.

**Alternatives considered**:
- Optimistic locking with version field: adds complexity (retry loops) for no benefit when `FOR UPDATE SKIP LOCKED` is available.
- Distributed lock (external): violates constitution Principle II (zero external dependencies in core).

## R-003: Worker Architecture (Distributed vs In-Process)

**Decision**: Single binary with `--mode` flag: `gateway`, `worker`, `integrated`. Gateway and worker share the same codebase and PostgreSQL.

**Rationale**: The brainstorm settled on Option B (separate Worker Deployment). Same binary, different modes keeps the build simple while enabling independent scaling in production. `integrated` mode combines both for development convenience.

**Alternatives considered**:
- In-process goroutine pool only (Option A): background work competes with sync requests, no resource isolation.
- K8s Jobs per request (Option C): cold start latency, RBAC complexity, overkill for the common case.
- External job queue (Option D): violates constitution Principle II.

## R-004: Background Request Serialization

**Decision**: Persist the full `CreateResponseRequest` alongside the response record when `background: true`. Workers reconstruct the request from storage to invoke the engine.

**Rationale**: The worker needs the original request (model, input, tools, instructions, etc.) to process the background request. Storing the serialized request as a JSON column alongside the response ensures the worker has everything it needs without additional infrastructure. The request is immutable after creation.

**Alternatives considered**:
- Store only a reference and replay from client: requires client cooperation, defeats fire-and-forget.
- Separate request table: adds schema complexity. A single JSON column on the response record is simpler and sufficient.

## R-005: Worker Heartbeat and Stale Detection

**Decision**: Workers update a `worker_heartbeat` timestamp on their claimed responses periodically. During each poll cycle, workers also check for responses where `status = 'in_progress'` and `worker_heartbeat` is older than the staleness timeout, marking them as `failed`.

**Rationale**: A crashed worker stops sending heartbeats. Other workers detect the staleness during their regular poll cycle. This avoids the need for a separate cleanup process and ensures stale detection scales with the number of workers.

**Alternatives considered**:
- Worker registration with health checks: adds infrastructure complexity (worker registry, health endpoints). Heartbeat on the response record is simpler.
- Time-since-claim without heartbeat: a long-running legitimate request would be incorrectly marked as stale. Periodic heartbeats distinguish alive-but-slow from crashed.

## R-006: TTL Cleanup Strategy

**Decision**: Workers run TTL cleanup as part of their poll cycle (piggyback pattern). On each poll, after claiming work, the worker deletes responses where `background = true` and terminal status (`completed`, `failed`, `cancelled`) and `created_at` is older than the configured TTL.

**Rationale**: No separate cron job or cleanup process needed. Workers already poll periodically, so adding cleanup to the poll cycle is natural. The cleanup is idempotent and bounded (deletes a batch per cycle to avoid long transactions).

**Alternatives considered**:
- Separate cleanup goroutine/process: adds operational complexity for a periodic delete.
- Database-level TTL (PostgreSQL `pg_cron`): external dependency, not portable to in-memory storage.

## R-007: Context Propagation for Cancellation

**Decision**: Workers create a cancellable context per background request. The cancel function is stored in an in-process registry keyed by response ID. When DELETE /v1/responses/{id} is called on the gateway, the gateway updates the response status to `cancelled` in storage. Workers check for cancellation during their processing loop by periodically re-reading the status from storage.

**Rationale**: In a distributed architecture, the gateway and worker may be different processes. Direct context cancellation (as used for streaming) only works in-process. For distributed cancellation, the worker must poll for status changes. This is simple and reliable, though slightly slower than in-process cancellation. For integrated mode, direct context cancellation via the in-process registry is also available as an optimization.

**Alternatives considered**:
- Direct RPC from gateway to worker for cancellation: requires worker discovery and a control plane. Overkill for MVP.
- Shared in-memory cancellation registry: only works in integrated mode, not distributed.
