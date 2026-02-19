# Feature Specification: Container Image Build

**Feature Branch**: `009-container-image`
**Created**: 2026-02-19
**Status**: Draft

## Overview

Create a multi-stage Containerfile for building the antwort server as a minimal, production-ready container image. Add Makefile targets for building and pushing images.

## User Scenarios & Testing

### User Story 1 - Build Container Image (Priority: P1)

A developer builds a container image for the antwort server using a single command. The image contains only the static Go binary on a distroless base for minimal attack surface.

**Acceptance Scenarios**:

1. **Given** the project source, **When** `make image-build` is run, **Then** a container image is built with the antwort server binary
2. **Given** a built image, **When** `podman run` is executed with required env vars, **Then** the server starts and responds on /healthz
3. **Given** the image, **When** inspected, **Then** it uses a distroless base with no shell, no package manager, and no unnecessary files

## Requirements

- **FR-001**: The project MUST provide a Containerfile with multi-stage build (Go builder + distroless runtime)
- **FR-002**: The Makefile MUST provide `image-build`, `image-push`, and `image-latest` targets
- **FR-003**: The image MUST expose port 8080 and run as non-root
- **FR-004**: The image tag MUST default to the git short SHA for traceability

## Success Criteria

- **SC-001**: `make image-build` produces a runnable container image
- **SC-002**: The image starts and serves /healthz correctly

## Assumptions

- Podman is the container runtime (per constitution).
- The image repository defaults to `ghcr.io/rhuss/antwort` but is configurable via `IMAGE_REPO`.

## Dependencies

- **Spec 006 (Conformance)**: The `cmd/server/main.go` that the image builds.
