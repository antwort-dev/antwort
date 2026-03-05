# Antwort - OpenResponses Gateway
# ================================

PROFILE   ?= core
BIN_DIR   := bin
IMAGE_REPO ?= ghcr.io/rhuss/antwort
SANDBOX_IMAGE_REPO ?= ghcr.io/rhuss/antwort-sandbox
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

KUBE_NAMESPACE ?= antwort

.PHONY: build test vet conformance api-test ci-sdk-test e2e coverage coverage-unit coverage-e2e coverage-all clean image-build image-push image-latest sandbox-build sandbox-push deploy deploy-openshift docs docs-serve

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

# Run SDK client tests (Python + TypeScript) against a local server.
# Requires: pip install -r test/sdk/python/requirements.txt
#           cd test/sdk/typescript && bun install
ci-sdk-test: build
	@MOCK_PORT=9090 $(BIN_DIR)/mock-backend & MOCK_PID=$$!; \
	ANTWORT_BACKEND_URL="http://localhost:9090" ANTWORT_MODEL="mock-model" ANTWORT_PORT="8080" ANTWORT_STORAGE="memory" $(BIN_DIR)/server & SERVER_PID=$$!; \
	for i in $$(seq 1 30); do curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && break; sleep 1; done; \
	pytest test/sdk/python/ -v; PYTEST_EXIT=$$?; \
	cd test/sdk/typescript && bun test; BUN_EXIT=$$?; \
	kill $$MOCK_PID $$SERVER_PID 2>/dev/null; \
	exit $$(( PYTEST_EXIT + BUN_EXIT ))

# Build sandbox container image.
sandbox-build:
	podman build -t $(SANDBOX_IMAGE_REPO):$(IMAGE_TAG) -f Containerfile.sandbox .

# Push sandbox container image.
sandbox-push: sandbox-build
	podman push $(SANDBOX_IMAGE_REPO):$(IMAGE_TAG)

# Run E2E tests locally (starts replay-backend + antwort).
e2e: build
	@echo "Starting replay backend..."
	@MOCK_PORT=9090 $(BIN_DIR)/mock-backend --recordings-dir test/e2e/recordings & \
	MOCK_PID=$$!; \
	echo "Starting antwort..."; \
	ANTWORT_BACKEND_URL=http://localhost:9090 ANTWORT_MODEL=mock-model ANTWORT_PORT=8080 ANTWORT_STORAGE=memory $(BIN_DIR)/server & \
	SERVER_PID=$$!; \
	sleep 2; \
	echo "Running E2E tests..."; \
	go test -tags e2e ./test/e2e/ -v -timeout 60s; \
	EXIT_CODE=$$?; \
	kill $$MOCK_PID $$SERVER_PID 2>/dev/null; \
	exit $$EXIT_CODE

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
	cd docs && npx antora antora-playbook.yml

# Build and serve documentation locally.
docs-serve: docs
	@echo "Documentation built at docs/build/site/"
	@echo "Starting local server at http://localhost:8070"
	npx http-server docs/build/site -p 8070 -c-1

COVERAGE_DIR := coverage

# Generate test coverage reports (unit + integration combined).
coverage:
	@mkdir -p $(COVERAGE_DIR)
	@echo "Running unit + integration tests with coverage..."
	go test $$(go list ./pkg/... | grep -v /postgres) ./test/integration/... \
		-coverpkg=./pkg/... \
		-coverprofile=$(COVERAGE_DIR)/combined.out \
		-timeout 120s -count=1
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=$(COVERAGE_DIR)/combined.out | tail -1
	@echo ""
	@echo "Generating HTML report..."
	@go tool cover -html=$(COVERAGE_DIR)/combined.out -o $(COVERAGE_DIR)/combined.html
	@echo "HTML report: $(COVERAGE_DIR)/combined.html"
	@# Extract percentage for badge
	@go tool cover -func=$(COVERAGE_DIR)/combined.out | tail -1 | awk '{print $$3}' > $(COVERAGE_DIR)/percentage.txt
	@echo "Coverage: $$(cat $(COVERAGE_DIR)/percentage.txt)"

# Generate E2E coverage using Go binary instrumentation (Go 1.20+).
# Builds an instrumented server binary, runs it with the mock backend,
# executes E2E tests, then extracts coverage from the binary's output.
# Uses deterministic mock (no replay) to avoid recording hash alignment issues.
coverage-e2e:
	@rm -rf $(COVERAGE_DIR)/e2e-raw && mkdir -p $(COVERAGE_DIR)/e2e-raw
	@echo "Building instrumented server binary..."
	go build -cover -o $(BIN_DIR)/server-cover ./cmd/server/
	go build -o $(BIN_DIR)/mock-backend ./cmd/mock-backend/
	@echo "Starting replay backend with recordings..."
	@MOCK_PORT=9091 $(BIN_DIR)/mock-backend --recordings-dir test/e2e/recordings & \
	MOCK_PID=$$!; \
	COV_ABS=$$(pwd)/$(COVERAGE_DIR)/e2e-raw; \
	echo "Starting instrumented antwort (GOCOVERDIR=$$COV_ABS)..."; \
	GOCOVERDIR=$$COV_ABS \
	ANTWORT_BACKEND_URL=http://localhost:9091 ANTWORT_MODEL=mock-model \
	ANTWORT_PORT=8081 ANTWORT_STORAGE=memory \
	$(BIN_DIR)/server-cover & \
	SERVER_PID=$$!; \
	sleep 2; \
	echo "Running E2E tests against instrumented binary..."; \
	ANTWORT_BASE_URL=http://localhost:8081/v1 \
	go test -tags e2e ./test/e2e/ -v -timeout 60s -count=1 \
		-run 'TestE2E' ; \
	E2E_EXIT=$$?; \
	echo "Stopping servers (SIGINT for coverage flush)..."; \
	kill -INT $$SERVER_PID 2>/dev/null; \
	for i in $$(seq 1 10); do kill -0 $$SERVER_PID 2>/dev/null || break; sleep 0.5; done; \
	kill $$MOCK_PID 2>/dev/null; sleep 1; \
	echo "Extracting E2E coverage..."; \
	go tool covdata textfmt -i=$(COVERAGE_DIR)/e2e-raw -o $(COVERAGE_DIR)/e2e.out 2>/dev/null; \
	if [ -f $(COVERAGE_DIR)/e2e.out ]; then \
		go tool cover -func=$(COVERAGE_DIR)/e2e.out | tail -1; \
		go tool cover -html=$(COVERAGE_DIR)/e2e.out -o $(COVERAGE_DIR)/e2e.html; \
		echo "E2E HTML report: $(COVERAGE_DIR)/e2e.html"; \
	else \
		echo "Warning: no E2E coverage data produced"; \
	fi; \
	exit $$E2E_EXIT

# Generate combined coverage from all three layers (unit + integration + E2E).
# Runs all test types and merges their coverage profiles.
coverage-all: coverage coverage-e2e
	@echo ""
	@echo "=== Merging all coverage layers ==="
	@mkdir -p $(COVERAGE_DIR)/all-raw
	@# Convert text profiles to covdata format for merging
	@if [ -f $(COVERAGE_DIR)/combined.out ] && [ -f $(COVERAGE_DIR)/e2e.out ]; then \
		go tool cover -func=$(COVERAGE_DIR)/combined.out | tail -1 | sed 's/total.*statements)/Unit+Integration:/' ; \
		go tool cover -func=$(COVERAGE_DIR)/e2e.out | tail -1 | sed 's/total.*statements)/E2E:            /' ; \
		echo ""; \
		echo "Merging profiles..."; \
		head -1 $(COVERAGE_DIR)/combined.out > $(COVERAGE_DIR)/all.out; \
		tail -n +2 $(COVERAGE_DIR)/combined.out >> $(COVERAGE_DIR)/all.out; \
		tail -n +2 $(COVERAGE_DIR)/e2e.out >> $(COVERAGE_DIR)/all.out; \
		go tool cover -func=$(COVERAGE_DIR)/all.out | tail -1 | sed 's/total.*statements)/Combined:       /' ; \
		go tool cover -html=$(COVERAGE_DIR)/all.out -o $(COVERAGE_DIR)/all.html; \
		echo ""; \
		echo "Combined HTML report: $(COVERAGE_DIR)/all.html"; \
	else \
		echo "Missing coverage profiles. Run make coverage and make coverage-e2e first."; \
	fi

# Generate coverage for unit tests only.
coverage-unit:
	@mkdir -p $(COVERAGE_DIR)
	go test ./pkg/... -coverprofile=$(COVERAGE_DIR)/unit.out -timeout 60s -count=1
	@go tool cover -func=$(COVERAGE_DIR)/unit.out | tail -1
	@go tool cover -html=$(COVERAGE_DIR)/unit.out -o $(COVERAGE_DIR)/unit.html

# Clean build artifacts.
clean:
	rm -rf $(BIN_DIR) $(COVERAGE_DIR) docs/build
