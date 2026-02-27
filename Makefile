.PHONY: build test lint clean install help test-all test-integration test-e2e test-coverage coverage docker-up docker-down

BINARY_NAME=dbdiff
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X=main.version=${VERSION} -X=main.commit=${COMMIT} -X=main.buildDate=${BUILD_DATE}"

help:
	@echo "DBDiff Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build             Build binary"
	@echo "  test              Run unit tests"
	@echo "  test-coverage     Run tests with coverage"
	@echo "  test-all          Run all tests (unit + integration)"
	@echo "  test-integration  Run integration tests (requires Docker)"
	@echo "  test-e2e          Run E2E tests"
	@echo "  lint              Run linter"
	@echo "  lint-fix          Run linter with auto-fix"
	@echo "  coverage          Generate coverage report"
	@echo "  docker-up         Start test containers"
	@echo "  docker-down       Stop test containers"
	@echo "  clean             Clean build artifacts"
	@echo "  install           Install binary"
	@echo "  release           Create release (requires tag)"

build:
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o ${BINARY_NAME} .
	@echo "Build complete: ./${BINARY_NAME}"

test:
	go test -v -race -coverprofile=coverage.out ./...

test-all: test test-integration

test-integration:
	@echo "Running integration tests..."
	@if ! command -v docker-compose &> /dev/null; then \
		echo "docker-compose not found. Skipping integration tests."; \
		exit 0; \
	fi
	./run_integration_tests.sh

test-e2e:
	@echo "Running E2E tests..."
	go test -tags=e2e -v ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

coverage: test-coverage

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run ./... --fix

docker-up:
	docker-compose up -d
	@echo "Waiting for databases..."
	sleep 10

docker-down:
	docker-compose down -v

clean:
	rm -f ${BINARY_NAME}
	rm -f coverage.out coverage.html

install:
	go install ${LDFLAGS}

release:
	goreleaser release --clean
