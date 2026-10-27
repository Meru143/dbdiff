# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Security
- **Migration files written by earlier versions hold the database passwords in their header** (`-- Source: postgres://user:password@host/db`, and the same for `-- Target:`). Scrub the ones you committed or shared, and rotate the passwords they name. dbdiff now hides the password wherever it prints a connection string, and keeps the user and host: the migration header of `migrate` and `interactive`, the `-v` logs of `compare`, `diff`, `migrate` and `tables`, `validate`'s output, and errors that quoted a URL or part of its password (an unsupported scheme, a PostgreSQL or SQL Server URL that does not parse, a URL given where a command or a flag value belongs) show `***` in its place. That holds for URL passwords in every form (`postgres://`, `sqlserver://`, `mysql://user:pass@tcp(host:3306)/db`, with `@`, `:`, `/` or %-escapes in them) and for `password=` and `pwd=` parameters. The `--executable` script no longer prints its database URL when it can't connect

### Fixed
- Schema comparisons leave out `_dbdiff_migrations`, the table `dbdiff apply` records applied migrations in, with its grants and triggers, whatever the case of its name: once a migration had been applied, `compare`, `diff`, `migrate` and `interactive` reported the table as a difference, and the next migration dropped it along with the migration history
- The header of `migrate` and `interactive` migrations names the schema actually read: `dbo` on SQL Server when `--schema` is left at its default, and on MySQL no schema line, since each side then reads its connection's own database, named in the URLs. Both used to print `-- Schema: public`

### Security
- pgx is upgraded from v5.8.0 to v5.11.0. v5.8.0 has three vulnerabilities, fixed in v5.9.0 and v5.9.2: GO-2026-4771, GO-2026-4772 and GO-2026-5004 (SQL injection via placeholder confusion with dollar-quoted strings)
- Release binaries are built with Go 1.26.10, from the committed tree. The v1.1.1 release ran `go mod tidy` before building, which rewrote `go.mod`: its binaries were built with Go 1.26.0, which has standard-library vulnerabilities fixed in later patch releases, and `go version -m` reports them as `v1.1.1+dirty`. `go.mod` is now tidy (CI checks it) and its `toolchain go1.26.10` directive sets the Go that CI and releases build with

## [1.1.1] - 2026-10-27

### Fixed
- PostgreSQL type changes drop the column's old default first, if it has one, and restore the source's after, since the old one may not cast to the new type (an identity column has none); MySQL string defaults double their backslashes; functions are dropped before the types their signatures use; and MySQL index definitions name the database when `--schema` picks one
- `dbdiff --help` opens with a description naming PostgreSQL, MySQL and SQL Server: the root command still said "PostgreSQL schema comparison and migration tool", and the help template left every command's description out; `--timeout` shows its real default, 30s, instead of 30ns, and `--schema` says what MySQL and SQL Server read by default
- Indexes: a uniqueness-only change is detected; an index that backs a primary key or unique constraint is compared and migrated through its constraint, so adding a UNIQUE constraint no longer also emits a colliding `CREATE UNIQUE INDEX` (MySQL unique keys are compared as indexes); and new tables get their indexes
- MySQL and SQL Server indexes keep their full definition (MySQL prefix lengths, descending and functional key parts, FULLTEXT and SPATIAL; SQL Server descending keys, `INCLUDE` columns, filters and `CLUSTERED`), so changes to them are detected and a recreated index matches the original; SQL Server no longer mixes `INCLUDE` columns into the key, and a table with a MySQL functional or PostgreSQL expression index no longer fails introspection
- Constraints whose definitions both servers report are compared by them, so a changed CHECK expression or UNIQUE column list shows
- Migration statements run in dependency order instead of the order the diffs were found in: foreign keys, constraints and indexes are dropped before the columns and tables they cover (SQL Server refuses to drop or alter an indexed column, and MySQL drops a column's index with it); sequences and types exist before the tables that use them; new tables are created before the foreign keys between them, which are now added separately; a recreated index, constraint or foreign key is dropped before its columns change and added back after; and a statement several diffs need runs once
- `migrate`, `interactive` and `compare` write SQL in the target database's dialect; they used to emit PostgreSQL SQL for MySQL and SQL Server, and qualified names with the connection URL when one was set
- Migrations use each dialect's lock timeout (`lock_timeout`, `lock_wait_timeout`, `SET LOCK_TIMEOUT`) and transaction statements (SQL Server: `SET XACT_ABORT ON` and `BEGIN TRANSACTION`); `apply` and `rollback` wrap files the same way, and on MySQL can run a multi-statement file
- CREATE TABLE, ADD COLUMN, constraints, foreign keys, sequences and enum types are written from the source schema's definitions instead of `CREATE TABLE t ();`, a `text` column or a bare `ADD CONSTRAINT name`; a change without a definition becomes a comment instead of a guess, and a sequence's start change is `START WITH n` (it was `ALTER SEQUENCE s n`)
- Introspection keeps full column types (`varchar(255)`, `decimal(10,2)`), identity and auto-increment, and constraint definitions; PostgreSQL no longer reports its OID-named not-null pseudo-constraints or foreign keys as constraints, nor identity columns' internal sequences, and MySQL no longer mixes other tables' primary key columns into a table's
- MySQL nullability and default changes are a full `MODIFY COLUMN` that keeps the column's type, default and `AUTO_INCREMENT` (it used to fall back to `VARCHAR(255)`); SQL Server column changes restate the type and nullability, drop and restore a column's DEFAULT constraint around type changes and column drops, use `ADD` instead of `ADD COLUMN`, and drop tables without `CASCADE`
- PostgreSQL default changes use `SET DEFAULT`/`DROP DEFAULT` (they were emitted as a type change), and a column order change is reported as a comment instead of an invalid `ALTER COLUMN`
- MySQL and SQL Server read their own default schema when `--schema` is left at `public`, and within one engine column types compare exactly, so `int` to `bigint` is no longer hidden by the type normalizer
- `migrate` and `interactive` refuse source and target on different engines, and `--executable` refuses non-PostgreSQL targets, instead of writing SQL the target can't run
- Index migrations: `CREATE INDEX` now has the index name, columns and uniqueness (PostgreSQL reuses the server's own index definition), `DROP INDEX` uses each dialect's syntax (`DROP INDEX … ON table` for MySQL and SQL Server), and PostgreSQL's `CONCURRENTLY` is only used when the migration isn't wrapped in a transaction

## [1.1.0] - 2026-03-11

### Added
- Interactive TUI mode (#5)
- Schema versioning and migration history (#9)
- Executable migration script generation (#8)
- Support for GRANT/REVOKE permissions (#4)
- Support for Triggers (#3)
- Support for Functions and Stored Procedures (#2)
- Support for Materialized Views (#6)
- Support for PostgreSQL Views (#1)

### Changed
- Moved test files from root to `test/` directory for better organization

### Fixed
- Function diff sorting and switch logic issues
- Error handling in shell completions generation

## [0.1.0] - 2024-02-26

### Added
- Initial release
- Schema comparison between two PostgreSQL databases
- Migration SQL generation
- Multiple output formats (SQL, Table, JSON)
- Dry-run mode (default)
- Schema filtering
- Ignore patterns for columns
- Transaction wrapper support
- SSL/TLS connection support
- Shell completions (bash, zsh, fish, PowerShell)
- Structured logging
- CI/CD with GitHub Actions
