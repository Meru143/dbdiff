# DBDiff

[![Build](https://github.com/meru143/dbdiff/actions/workflows/ci.yml/badge.svg)](https://github.com/meru143/dbdiff/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/meru143/dbdiff/branch/main/graph/badge.svg)](https://codecov.io/gh/meru143/dbdiff)
[![Release](https://img.shields.io/github/v/release/meru143/dbdiff)](https://github.com/meru143/dbdiff/releases)

PostgreSQL schema comparison and migration CLI tool.

## Features

- Compare PostgreSQL schemas between two databases
- Generate migration SQL scripts
- Multiple output formats: SQL, Table, JSON
- Dry-run mode (default)
- Schema filtering
- Ignore patterns for columns
- Transaction wrapper support
- SSL/TLS connection support

## Installation

```bash
# Via Go install
go install github.com/meru143/dbdiff@latest

# Via Homebrew
brew tap meru143/dbdiff
brew install dbdiff

# Via binary
curl -L https://github.com/meru143/dbdiff/releases/latest/download/dbdiff-linux-amd64 -o dbdiff
chmod +x dbdiff
```

## Usage

```bash
# Compare two databases
dbdiff compare postgres://user:pass@localhost:5432/db1 postgres://user:pass@localhost:5432/db2

# Generate migration
dbdiff migrate -s postgres://localhost:5432/db1 -t postgres://localhost:5432/db2 -o migration.sql

# Show table diff
dbdiff diff postgres://localhost:5432/db1 postgres://localhost:5432/db2 --format table

# List tables
dbdiff tables postgres://localhost:5432/db

# Validate connection
dbdiff validate postgres://localhost:5432/db
```

## Configuration

Create `config.yaml`:

```yaml
source: postgres://user:pass@localhost:5432/db1
target: postgres://user:pass@localhost:5432/db2
output: ./migrations/
format: sql
schema: public
ignore_patterns:
  - "_created_at"
  - "_updated_at"
```

## License

MIT License - see [LICENSE](LICENSE)
