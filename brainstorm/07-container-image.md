# Spec 07a: Container Image

**Branch**: `spec/07a-container-image`
**Dependencies**: Spec 06 (Conformance, for cmd/server)
**Package**: N/A (Containerfile + Makefile targets)

## Purpose

Define the container image build for the antwort server binary. Multi-stage build with distroless base for minimal attack surface.

## Scope

### In Scope
- Multi-stage Containerfile (builder + distroless runtime)
- Makefile targets for build and push
- Multi-architecture support (amd64 + arm64)
- Image tagging strategy (git SHA, semver, latest)

### Out of Scope
- Kubernetes manifests (separate spec)
- Helm chart (separate spec)
- CI/CD pipeline (project-specific)

## Container Image

```dockerfile
# Multi-stage build
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /antwort ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /antwort /antwort
EXPOSE 8080
ENTRYPOINT ["/antwort"]
```

## Makefile Targets

```makefile
IMAGE_REPO ?= ghcr.io/rhuss/antwort
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD)

image-build:
	podman build -t $(IMAGE_REPO):$(IMAGE_TAG) -f Containerfile .

image-push:
	podman push $(IMAGE_REPO):$(IMAGE_TAG)

image-latest: image-build
	podman tag $(IMAGE_REPO):$(IMAGE_TAG) $(IMAGE_REPO):latest
```

## Deliverables

- [ ] `Containerfile` - Multi-stage build for server
- [ ] `Makefile` - image-build, image-push, image-latest targets
