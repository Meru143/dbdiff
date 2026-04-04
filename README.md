# DBDiff

[![Build](https://github.com/meru143/dbdiff/actions/workflows/ci.yml/badge.svg)](https://github.com/meru143/dbdiff/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/meru143/dbdiff)](https://github.com/meru143/dbdiff/releases)
[![Coverage](https://codecov.io/gh/meru143/dbdiff/branch/main/graph/badge.svg)](https://codecov.io/gh/meru143/dbdiff)
[![Go Version](https://img.shields.io/github/go-mod/go-version/meru143/dbdiff)](https://github.com/meru143/dbdiff)

Multi-database schema comparison and migration CLI tool. Supports **PostgreSQL**, **MySQL**, and **SQL Server**.

Live page: [merup.me/dbdiff](https://merup.me/dbdiff/)

DBDiff keeps schema comparison and migration generation in one CLI so you can inspect drift, review the generated SQL, and move forward without bouncing between separate tools or dialect-specific scripts.

## Features

- Compare database schemas between two instances
- Generate migration SQL scripts
- **Multi-database support**: PostgreSQL and MySQL
- Dialect-aware SQL generation (e.g., `MODIFY COLUMN` for MySQL, `ALTER COLUMN TYPE` for PostgreSQL)
- Multiple output formats: SQL, Table, JSON
- Dry-run mode (default)
- Schema filtering
- Ignore patterns for columns
- Transaction wrapper support
- SSL/TLS connection support
- Migration tracking

## Installation

```bash
# Via Go install
go install github.com/meru143/dbdiff@latest

# Via Homebrew
brew install meru143/homebrew-dbdiff/dbdiff

# Via binary
curl -L https://github.com/meru143/dbdiff/releases/latest/download/dbdiff-linux-amd64 -o dbdiff
chmod +x dbdiff
```

## Usage

### PostgreSQL

```bash
# Compare two PostgreSQL databases
dbdiff compare postgres://user:pass@localhost:5432/db1 postgres://user:pass@localhost:5432/db2

# Generate migration
dbdiff migrate -s postgres://localhost:5432/db1 -t postgres://localhost:5432/db2 -o migration.sql

# Show table diff
dbdiff diff postgres://localhost:5432/db1 postgres://localhost:5432/db2 --format table
```

### MySQL

```bash
# Compare two MySQL databases
dbdiff compare mysql://user:pass@tcp(localhost:3306)/db1 mysql://user:pass@tcp(localhost:3306)/db2

# Generate migration
dbdiff migrate -s mysql://user:pass@tcp(localhost:3306)/db1 -t mysql://user:pass@tcp(localhost:3306)/db2 -o migration.sql
```

### SQL Server

```bash
# Compare two SQL Server databases
dbdiff compare "sqlserver://sa:Password@localhost:1433?database=db1" "sqlserver://sa:Password@localhost:1433?database=db2"

# Generate migration
dbdiff migrate -s "sqlserver://sa:Password@localhost:1433?database=db1" -t "sqlserver://sa:Password@localhost:1433?database=db2" -o migration.sql
```

### General Commands

```bash
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

## Supported Databases

| Database       | Introspection | Migration SQL | Dialect-Aware DDL |
|----------------|:---:|:---:|:---:|
| PostgreSQL     | ✅ | ✅ | ✅ |
| MySQL 8.0+     | ✅ | ✅ | ✅ |
| SQL Server     | ✅ | ✅ | ✅ |

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| --source | -s | Source database URL | |
| --target | -t | Target database URL | |
| --output | -o | Output file path | stdout |
| --format | | Output format: sql, table, json | sql |
| --schema | | Database schema | public |
| --dry-run | | Dry-run mode (don't write) | true |
| --force | -f | Skip confirmation prompt | false |
| --timeout | | Query timeout | 30s |
| --transaction | | Wrap in transaction | true |
| --ssl-mode | | SSL mode: disable, require, verify-ca, verify-full | disable |
| --backup-dir | | Backup directory for migrations | |
| --max-backups | | Maximum backups to keep | 5 |
| --protected-objects | | Protected objects (skip in migration) | |
| --ignore-patterns | | Columns to ignore | |
| --verbose | -v | Verbose output | |
| --debug | | Debug output | |
| --config | -c | Config file path | |

## Environment Variables

| Variable | Description |
|----------|-------------|
| DBDIFF_SOURCE | Source database URL |
| DBDIFF_TARGET | Target database URL |
| DBDIFF_LOG_LEVEL | Log level: debug, info, warn, error |
| DBDIFF_CONFIG | Config file path |

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires Docker)
docker compose -f docker-compose.test.yml up -d
go run -tags=integration test/integration/main.go
docker compose -f docker-compose.test.yml down
```

## License

MIT License - see [LICENSE](LICENSE)
