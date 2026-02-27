.PHONY: build test lint clean install help

BINARY_NAME=dbdiff
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X=main.version=${VERSION} -X=main.commit=${COMMIT} -X=main.buildDate=${BUILD_DATE}"

help:
	@echo "DBDiff Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build binary"
	@echo "  test        Run tests"
	@echo "  test-coverage Run tests with coverage"
	@echo "  lint        Run linter"
	@echo "  clean       Clean build artifacts"
	@echo "  install     Install binary"

build:
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o ${BINARY_NAME} .
	@echo "Build complete: ./${BINARY_NAME}"

test:
	go test -v -race -coverprofile=coverage.out ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

clean:
	rm -f ${BINARY_NAME}
	rm -f coverage.out coverage.html

install:
	go install ${LDFLAGS}
