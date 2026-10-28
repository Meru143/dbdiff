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
- **Multi-database support**: PostgreSQL, MySQL 8+ and SQL Server
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

# Via release binary (Linux x86-64 shown)
curl -fsSL https://github.com/Meru143/dbdiff/releases/download/v1.1.4/dbdiff_1.1.4_linux_amd64.tar.gz | tar -xz dbdiff
sudo mv dbdiff /usr/local/bin/
```

Release archives are named `dbdiff_<version>_<os>_<arch>`: `.tar.gz` for `linux` and `darwin`, `.zip` for `windows`, each in `amd64` and `arm64`. They are listed with `checksums.txt` on the [releases page](https://github.com/Meru143/dbdiff/releases/latest).

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
| --schema | | Database schema (MySQL: the connection's database, SQL Server: `dbo`, unless set) | public |
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

`dbdiff migrate` writes SQL for the target's engine, so source and target must run the same one. Column types, defaults, constraints and foreign keys come from the source database's own definitions. With `--transaction` (the default) the migration runs in one transaction: `BEGIN`/`COMMIT` on PostgreSQL, `SET XACT_ABORT ON` with `BEGIN TRANSACTION`/`COMMIT TRANSACTION` on SQL Server, and `START TRANSACTION`/`COMMIT` on MySQL, where every DDL statement commits implicitly, so a migration that fails partway keeps the statements before the failure. `--executable` writes a `psql` script, so it is PostgreSQL-only.

With `--transaction=false`, PostgreSQL index changes use `CREATE INDEX CONCURRENTLY` and `DROP INDEX CONCURRENTLY`, so they don't block writes. PostgreSQL can't run those inside a transaction, so apply that migration with `psql -f migration.sql`: `dbdiff apply` sends the whole file as one batch, which PostgreSQL runs as a single transaction.

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
