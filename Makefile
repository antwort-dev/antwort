# Antwort - OpenResponses Gateway
# ================================

PROFILE   ?= core
BIN_DIR   := bin
IMAGE_REPO ?= ghcr.io/rhuss/antwort
SANDBOX_IMAGE_REPO ?= ghcr.io/rhuss/antwort-sandbox
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

KUBE_NAMESPACE ?= antwort

.PHONY: build test vet conformance api-test clean image-build image-push image-latest sandbox-build sandbox-push deploy deploy-openshift docs docs-serve

# Build all binaries.
build:
	go build -o $(BIN_DIR)/server ./cmd/server/
	go build -o $(BIN_DIR)/mock-backend ./cmd/mock-backend/
	go build -o $(BIN_DIR)/sandbox-server ./cmd/sandbox-server/

# Run all Go tests.
test:
	go test ./pkg/... -timeout 30s

# Run go vet.
vet:
	go vet ./pkg/... ./cmd/...

# Run API conformance pipeline (oasdiff + integration tests).
api-test:
	./test/run.sh

# Run OpenResponses conformance tests.
# Usage: make conformance PROFILE=core
#        make conformance PROFILE=extended
conformance: build
	./conformance/run.sh $(PROFILE)

# Build sandbox container image.
sandbox-build:
	podman build -t $(SANDBOX_IMAGE_REPO):$(IMAGE_TAG) -f Containerfile.sandbox .

# Push sandbox container image.
sandbox-push: sandbox-build
	podman push $(SANDBOX_IMAGE_REPO):$(IMAGE_TAG)

# Build container image.
image-build:
	podman build -t $(IMAGE_REPO):$(IMAGE_TAG) -f Containerfile .

# Push container image.
image-push: image-build
	podman push $(IMAGE_REPO):$(IMAGE_TAG)

# Tag and push as latest.
image-latest: image-build
	podman tag $(IMAGE_REPO):$(IMAGE_TAG) $(IMAGE_REPO):latest
	podman push $(IMAGE_REPO):latest

# Deploy to Kubernetes (base manifests).
deploy:
	kustomize build deploy/kubernetes/base/ | kubectl apply -n $(KUBE_NAMESPACE) -f -

# Deploy to OpenShift/ROSA (with Route).
deploy-openshift:
	kustomize build deploy/kubernetes/overlays/openshift/ | kubectl apply -n $(KUBE_NAMESPACE) -f -

# Build documentation site.
docs:
	@command -v npx >/dev/null 2>&1 || { echo "Error: npx not found. Install Node.js 18+ from https://nodejs.org"; exit 1; }
	@test -d docs/node_modules/@antora/lunr-extension || (cd docs && npm install --save-dev @antora/lunr-extension 2>/dev/null)
	npx antora docs/antora-playbook.yml

# Build and serve documentation locally.
docs-serve: docs
	@echo "Documentation built at docs/build/site/"
	@echo "Starting local server at http://localhost:8070"
	npx http-server docs/build/site -p 8070 -c-1

# Clean build artifacts.
clean:
	rm -rf $(BIN_DIR) docs/build
