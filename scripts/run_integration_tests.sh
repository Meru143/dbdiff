#!/bin/bash
set -e

echo "=== dbdiff Integration Test Runner ==="

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "Docker not found. Please install Docker."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "docker-compose not found. Please install docker-compose."
    exit 1
fi

# Start containers
echo "Starting test databases..."
# Ensure we are in project root for docker-compose file
docker-compose --project-directory . -f docker-compose.test.yml up -d

# Wait for databases to be ready
echo "Waiting for PostgreSQL and MySQL to be ready..."
sleep 20 # MySQL takes longer to boot up

# Run integration tests locally
echo "Running integration tests..."
go run -tags=integration test/integration/main.go

# Stop containers
echo "Stopping test databases..."
docker-compose --project-directory . -f docker-compose.test.yml down

echo "=== Integration tests complete ==="
