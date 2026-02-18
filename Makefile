# Antwort - OpenResponses Gateway
# ================================

PROFILE ?= core
BIN_DIR := bin

.PHONY: build test vet conformance clean

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

# Clean build artifacts.
clean:
	rm -rf $(BIN_DIR)
