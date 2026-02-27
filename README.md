# DBDiff

[![Build](https://github.com/meru143/dbdiff/actions/workflows/ci.yml/badge.svg)](https://github.com/meru143/dbdiff/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/meru143/dbdiff)](https://github.com/meru143/dbdiff/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/meru143/dbdiff)](https://github.com/meru143/dbdiff)

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
brew install meru143/homebrew-dbdiff/dbdiff

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

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| --source | -s | Source database URL | |
| --target | -t | Target database URL | |
| --output | -o | Output file path | stdout |
| --format | | Output format: sql, table, json | sql |
| --schema | | PostgreSQL schema | public |
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

## License

MIT License - see [LICENSE](LICENSE)
