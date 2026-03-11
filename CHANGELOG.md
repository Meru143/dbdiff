# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
