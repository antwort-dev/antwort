# Antwort - OpenResponses Gateway
# ================================

PROFILE   ?= core
BIN_DIR   := bin
IMAGE_REPO ?= ghcr.io/rhuss/antwort
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

KUBE_NAMESPACE ?= antwort

.PHONY: build test vet conformance clean image-build image-push image-latest deploy deploy-openshift

# Build server and mock-backend binaries.
build:
	go build -o $(BIN_DIR)/server ./cmd/server/
	go build -o $(BIN_DIR)/mock-backend ./cmd/mock-backend/

# Run all Go tests.
test:
	go test ./pkg/... -timeout 30s

# Run go vet.
vet:
	go vet ./pkg/... ./cmd/...

# Run OpenResponses conformance tests.
# Usage: make conformance PROFILE=core
#        make conformance PROFILE=extended
conformance: build
	./conformance/run.sh $(PROFILE)

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

# Clean build artifacts.
clean:
	rm -rf $(BIN_DIR)
