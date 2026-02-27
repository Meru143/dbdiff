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
docker-compose up -d

# Wait for databases to be ready
echo "Waiting for databases to be ready..."
sleep 10

# Run integration tests
echo "Running integration tests..."
cd /home/meru/workspace/dbdiff
go run -tags=integration integration_test.go

# Stop containers
echo "Stopping test databases..."
docker-compose down

echo "=== Integration tests complete ==="
