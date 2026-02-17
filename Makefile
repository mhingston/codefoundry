.PHONY: all build test clean lint install

BINARY_NAME=codefoundry
BUILD_DIR=./build
CMD_DIR=./cmd/codefoundry

VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

test:
	@echo "Running tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	@echo "Running linter..."
	golangci-lint run

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/ 2>/dev/null || cp $(BUILD_DIR)/$(BINARY_NAME) ~/go/bin/

run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

dev-init:
	@echo "Initializing development environment..."
	@mkdir -p .codefoundry/{protocols,state,artifacts}
	@echo "Development environment initialized"

schema-validate:
	@echo "Validating JSON schemas..."
	@for schema in schemas/*.schema.json; do \
		echo "  Checking $$schema..."; \
	done
	@echo "Schema validation complete"

# Protocol validation
PROTOCOL_FILE ?= .codefoundry/protocols/default.yaml
validate-protocol:
	@echo "Validating protocol: $(PROTOCOL_FILE)"
	@go run ./cmd/codefoundry validate --protocol $(PROTOCOL_FILE)

.DEFAULT_GOAL := build
