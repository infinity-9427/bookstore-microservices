# Orders Service Makefile

.PHONY: build test test-unit test-integration test-race clean

# Build optimized binary
build:
	go build -trimpath -ldflags "-s -w -buildid=" -tags netgo -o orders ./cmd/api

# Run all tests (unit tests by default, no external dependencies required)
test:
	go test ./... -race -count=1 -v

# Run only unit tests (fast, no external dependencies)
test-unit:
	go test ./... -race -count=1 -short

# Run integration tests (requires Docker for testcontainers)
test-integration:
	go test -tags=integration ./... -race -count=1 -v

# Test with race detection
test-race:
	go test ./... -race -count=1

# Run specific test pattern
test-orders:
	go test -run TestCreateOrder_ -v ./...

# Performance benchmarks
bench:
	go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	rm -f orders

# Development commands
dev-deps:
	go mod tidy
	go mod download

# Database migration (requires DATABASE_URL)
migrate:
	@if [ -z "$(DATABASE_URL)" ]; then echo "DATABASE_URL required"; exit 1; fi
	psql "$(DATABASE_URL)" -f migrations/004_idempotency_concurrent_index.sql

# Local development with test database
dev-test-db:
	docker run --rm -d \
		--name orders-test-db \
		-p 5432:5432 \
		-e POSTGRES_DB=orders_test \
		-e POSTGRES_USER=test \
		-e POSTGRES_PASSWORD=test \
		postgres:15-alpine

# Stop test database
stop-test-db:
	docker stop orders-test-db || true

# Full test suite with coverage
test-coverage:
	go test ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Lint and format
lint:
	go fmt ./...
	go vet ./...
	golangci-lint run || true

# Default target
all: clean lint test build