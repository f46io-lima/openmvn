.PHONY: all build test clean run stop help patch update-dockerfiles rebuild

# Variables
SERVICES = amf smf ocs upf bss udm
DOCKER_COMPOSE = docker-compose
GO = go

# Default target
all: build

# Patch Go modules
patch:
	@echo "🔧 Patching Go modules..."
	@chmod +x patch_all.sh
	@./patch_all.sh

# Update Dockerfiles
update-dockerfiles:
	@echo "🔧 Updating Dockerfiles..."
	@chmod +x update_dockerfiles.sh
	@./update_dockerfiles.sh

# Rebuild all services
rebuild: patch update-dockerfiles
	@echo "🏗️ Rebuilding all services..."
	$(DOCKER_COMPOSE) build --no-cache

# Build all services
build:
	@echo "Building all services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		cd $$service && $(GO) mod tidy && cd ..; \
	done
	$(DOCKER_COMPOSE) build

# Run tests
test:
	@echo "Running tests..."
	@for service in $(SERVICES); do \
		echo "Testing $$service..."; \
		cd $$service && $(GO) test -v ./... && cd ..; \
	done

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@for service in $(SERVICES); do \
		echo "Testing $$service with coverage..."; \
		cd $$service && $(GO) test -v -coverprofile=coverage.out ./... && \
		$(GO) tool cover -html=coverage.out -o coverage.html && cd ..; \
	done

# Start all services
run:
	@echo "Starting all services..."
	$(DOCKER_COMPOSE) up -d

# Stop all services
stop:
	@echo "Stopping all services..."
	$(DOCKER_COMPOSE) down

# Clean up
clean:
	@echo "Cleaning up..."
	$(DOCKER_COMPOSE) down -v
	@for service in $(SERVICES); do \
		rm -f $$service/coverage.out $$service/coverage.html; \
	done

# Initialize Redis with test data
init-redis:
	@echo "Initializing Redis with test data..."
	@redis-cli -h localhost -p 6379 flushall
	@redis-cli -h localhost -p 6379 hset "quota:001010123456789" "remaining" 1000 "updated" $$(date +%s)
	@redis-cli -h localhost -p 6379 set "ue:001010123456789" '{"imsi":"001010123456789","ip":"192.168.1.100","created":"2024-01-01T00:00:00Z","last_seen":"2024-01-01T00:00:00Z"}'
	@redis-cli -h localhost -p 6379 set "session:001010123456789" '{"ue_id":"001010123456789","teid":305419896,"ue_ip":"192.168.1.100","created":"2024-01-01T00:00:00Z","state":"active"}'
	@redis-cli -h localhost -p 6379 set "pfcp:305419896" '{"teid":305419896,"ue_ip":"192.168.1.100","created":"2024-01-01T00:00:00Z","state":"active","seid":1311768467294899695,"qfi":1,"priority":255}'

# Show help
help:
	@echo "Available targets:"
	@echo "  all              - Build all services (default)"
	@echo "  build            - Build all services"
	@echo "  patch            - Patch Go modules and update dependencies"
	@echo "  update-dockerfiles - Update all Dockerfiles to template"
	@echo "  rebuild          - Patch modules, update Dockerfiles, and rebuild all services"
	@echo "  test             - Run tests"
	@echo "  test-coverage    - Run tests with coverage"
	@echo "  run              - Start all services"
	@echo "  stop             - Stop all services"
	@echo "  clean            - Clean up"
	@echo "  init-redis       - Initialize Redis with test data"
	@echo "  help             - Show this help message" 