.PHONY: all build clean test run-amf run-smf run-upf run-bss run-ocs run-udm proto api-docs

# Build variables
BINARY_DIR := bin
SERVICES := amf smf upf bss ocs udm

# Go variables
GO := go
GOFMT := gofmt
GOLINT := golangci-lint
GOTEST := $(GO) test
GOBUILD := $(GO) build

# Default target
all: clean build

# Create binary directory
$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Build all services
build: $(BINARY_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GOBUILD) -o $(BINARY_DIR)/$$service ./cmd/$$service; \
	done

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)
	$(GO) clean

# Run tests
test:
	$(GOTEST) -v ./...

# Run individual services
run-amf: build
	$(BINARY_DIR)/amf

run-smf: build
	$(BINARY_DIR)/smf

run-upf: build
	$(BINARY_DIR)/upf

run-bss: build
	$(BINARY_DIR)/bss

run-ocs: build
	$(BINARY_DIR)/ocs

run-udm: build
	$(BINARY_DIR)/udm

# Format code
fmt:
	$(GOFMT) -w ./...

# Run linter
lint:
	$(GOLINT) run ./...

# Generate protobuf code
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		./api/proto/*.proto

# Generate API documentation
api-docs:
	@echo "Generating OpenAPI documentation..."
	# TODO: Add swagger generation commands

# Development environment
dev: build
	docker-compose up -d

# Stop development environment
dev-stop:
	docker-compose down

# Help target
help:
	@echo "Available targets:"
	@echo "  all        - Clean and build all services"
	@echo "  build      - Build all services"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  run-*      - Run individual services (amf, smf, upf, bss, ocs, udm)"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linter"
	@echo "  proto      - Generate protobuf code"
	@echo "  api-docs   - Generate API documentation"
	@echo "  dev        - Start development environment"
	@echo "  dev-stop   - Stop development environment" 