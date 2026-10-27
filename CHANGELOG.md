# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Fixed
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
