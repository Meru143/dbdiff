# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [1.1.5] - 2026-10-29

### Security
- **A raw `&` in a password parameter showed the rest of the password**: pgx and the SQL Server driver end a parameter's value at the next `&`, and so did dbdiff when it hid one, so `postgres://user@host/db?sslmode=disable&password=pass&word` was printed as `…&password=***&word`. That reached `validate`'s output and the `-v` and `--debug` logs, on PostgreSQL and SQL Server, and on PostgreSQL, when pgx couldn't read what followed the `&` (`&word`, `&a=b=c`), the error of every command that connects, twice: ``cannot parse `…&password=***&word`: failed to parse as URL (missing key/value separator "=" in URI query parameter: "word")``. It held for `password=`, `sslpassword=`, `pwd=` and the other password parameters, with or without a port, and for settings and DSNs given where a command or a flag value belongs: `server=host;password=pass&word;database=db` was printed as `server=host;password=***&word;database=db`. 1.1.4 and every earlier version do this. A driver can't tell an `&` in a password from the start of the next parameter, and neither can dbdiff, so it now hides all that follows a password parameter's `=`, to the end of the URL or the settings, or to the quote that closes them in an error (see Changed). The drivers read what followed the `&` as parameters, and when the server or the driver refused one, its error quoted it, as PostgreSQL's `unrecognized configuration parameter "word"` does. MySQL's driver has no password parameter: it sends one to the server as a `SET`, which never logs anyone in, and where the login needs no password the server's syntax error quoted its value, `&` or not (`near 'pass' at line 1`), even past the 80 characters the server cuts such an error to, or across a second parameter. dbdiff now refuses a password parameter in a `mysql://` or `mysql+tcp://` URL and says to put the password in the user info. For PostgreSQL and SQL Server, where a password parameter is real, dbdiff leaves out a connection error that quotes part of what it hides — a parameter name the server rejects, one padded with the spaces, tabs or control bytes a `%`-escape decoded to, a setting inside an `options` value, or go-mssqldb's unquoted duplicate key — and names the URL with that part hidden instead
- Migration files hold what followed the `&` only where such a URL connected: dbdiff writes one once both databases have accepted the connection, and the drivers sent only the part of the password before the `&`, which a server that checks that password refuses. Such a URL connected when nothing checked that password: on a PostgreSQL server with the `trust` method, with an `sslpassword=` and no encrypted client key to open with it, or with a SQL Server `pwd=`, which the driver ignores in a URL; and on PostgreSQL only when what followed the `&` read as settings pgx and the server take, as in `sslpassword=pass&application_name=word` or any dotted custom setting (`word.x=y`), which PostgreSQL accepts. The migration's `-- Source:` or `-- Target:` line then shows that part: if you wrote such a URL, scrub those lines, and change the password wherever it is checked
- A URL given in place of a file or a schema name was printed as given: `tables --schema "$URL"` printed `No tables found in schema: <URL>`, `migrate -o "$URL"` printed it in its prompt and its `-v` log line, `compare -o` and `interactive -o` did the same, and `migrate --backup-dir "$URL"` printed it in its `Backup created:` and `Failed to create backup:` `-v` lines. The password in it is hidden now. A URL given to `-o` or `--backup-dir` still becomes directories on disk named after it, with the password in those names; only what dbdiff prints is hidden. `--config "$URL"` printed what followed the last `.` in it, part of the password included, which viper took for the file's type (`Unsupported Config Type "word@host:5432"`); dbdiff no longer reads a URL given for `--config` or in `DBDIFF_CONFIG`, and says it takes a file

### Changed
- What dbdiff prints of a connection string, in the migration header, `-v` logs, `validate` and errors, now ends at a password parameter: `sqlserver://host?database=db&password=x&encrypt=disable` is printed as `sqlserver://host?database=db&password=***`, and `server=host;password=x;database=db` as `server=host;password=***`; a connection error that quotes one of the parameters after a password parameter is left out, as above. To keep the other parameters in view, put the password in the user-info part (`postgres://user:pass%26word@host/db?sslmode=disable`) or, on PostgreSQL and SQL Server, its parameter last; and in a parameter, write an `&` in the password as `%26`, or the driver reads only the part before it. A MySQL URL's password must be in the user info now: a password parameter there is refused, since go-sql-driver never logs in with one (so putting it last would not have protected it either). Connections don't change otherwise: the drivers still read a password parameter up to the next `&`, and dbdiff refuses the URLs 1.1.4 refused, a MySQL password parameter now too. A faulty parameter after a PostgreSQL password still reports why the URL won't parse (`sslmode is invalid`, `invalid port`), with the password hidden

## [1.1.4] - 2026-10-28

### Security
- Passwords still reached the terminal, logs and migration files in 1.1.3 in these cases, found by running every command with a password marked in each of its parts. A URL glued to a flag (`compare -v"$URL"`, `--source"$URL"`, `--"$URL"`) was quoted whole in the error for the unknown flag (`unknown shorthand flag: 'p' in -postgres://user:password@host/db`); passwords are now hidden in a URL wherever it starts. `dbdiff help "$URL"` printed it in ``Unknown help topic [`postgres://user:password@host/db`]``, and `dbdiff __complete "$URL" ''`, which shells run for completions, in `unable to find a command for arguments: [postgres://user:password@host/db]`, on stderr and in `$BASH_COMP_DEBUG_FILE`: cobra prints both itself, and dbdiff now hides the password in their arguments first. A URL whose password lacked the `@host` after it, as in `postgres://user:password`, was printed whole, in `-v` logs and in the error for it; what stands in the place of a port is now hidden unless it is a number. A password setting whose value holds a URL (`password=postgres://u:p;word@host/db`) was hidden only up to the `;`; all of it is hidden now. MySQL's errors for a URL without `/database` after the address quoted part of the password: `mysql://user:pass/word@tcp(host:3306)` failed with `default addr for network 'user:pass' unknown`, and `mysql://user:pass/word%ZZ@tcp(host:3306)` with `invalid dbname "word%ZZ@tcp(host:3306)"`. dbdiff now passes on only go-sql-driver's errors that quote nothing of the DSN, and for the others says what a MySQL URL looks like
- **A password parameter holding an `@`, after a port, was hidden only up to the `@`**: `sqlserver://host:1433?database=db&password=P@ssw0rd` was printed as `sqlserver://host:***@ssw0rd` in `-v` logs, errors and the migration header. Migration files written from such a URL hold the rest of the password: scrub them and rotate it. dbdiff now hides all of that value, and refuses such a URL (see below)
- **A password parameter holding a `;` was hidden only up to the `;`**: pgx reads `postgres://app@host/db?password=pass;word` as the password `pass;word`, which the `-v` logs and the migration header showed as `?password=***;word`. Migration files written from such a URL hold the rest of the password: scrub them and rotate it. In a URL, a parameter's value now runs to the next `&`, as the drivers read it
- dbdiff refuses a URL whose driver would read part of the password as the user name, the host or the database, which the driver's errors quote: `postgres://user:/pass@host:5432/db` failed with ``failed to connect to `user=me database=pass@host:5432/db` ``. Such URLs are a PostgreSQL URL with `@` or `/` in its user name or password, a SQL Server URL with `/`, `?` or `#` there, a MySQL URL without `/database` after the address, and one with a password parameter holding an `@` like the one above. The error names the URL with its password hidden and says what to percent-encode or add. A PostgreSQL or SQL Server URL with another `@` after its host, as in `?application_name=a@b`, can be refused too: write that `@` as `%40`
- A MySQL URL with a password and a raw `@` after its `/database`, as in `?connectionAttributes=owner:ops@example`, is refused too, though 1.1.3 connected with it: write that `@` as `%40`

### Fixed
- `mysql+tcp://` URLs, and MySQL URLs whose scheme is in upper case, connect as the user they name. dbdiff accepted both, but read the scheme as the user name: `Access denied for user 'mysql+tcp'`

## [1.1.3] - 2026-10-28

### Security
- **Migration files written by earlier versions hold the database passwords in their header** (`-- Source: postgres://user:password@host/db`, and the same for `-- Target:`). Scrub the ones you committed or shared, and rotate the passwords they name. dbdiff now hides the password wherever it prints a connection string, and keeps the user and host: the migration header of `migrate` and `interactive`, the `-v` logs of `compare`, `diff`, `migrate` and `tables`, `validate`'s output, and errors that quoted a URL or part of its password (a PostgreSQL or SQL Server URL that does not parse, a URL given where a command or a flag value belongs) show `***` in its place. The error for an unsupported scheme leaves the string out. That holds for URL passwords in every form (`postgres://`, `sqlserver://`, `mysql://user:pass@tcp(host:3306)/db`, with `@`, `:`, `/` or %-escapes in them) and for `password=`, `pwd=` and other password parameters, in a URL or in settings like `user id=sa;password=x`. The `--executable` script no longer prints its database URL when it can't connect

### Fixed
- `compare`, `diff`, `migrate` and `interactive` print "No differences found." on stderr when the schemas match. It was an info log, which only `-v` shows, so without it they printed nothing at all. `compare --format json` still prints its result on stdout, with `"diffs": []` (it was `null`)
- Logs go to stderr instead of stdout, where they landed in the SQL or JSON on stdout: `dbdiff migrate src tgt > migration.sql` wrote a migration ending in a JSON log line, which `apply` could not run. And `migrate` no longer logs "Dry-run mode: migration not applied" on every run: it only writes migrations, and never applies one
- Migration files end with a newline after their final `COMMIT;`
- `interactive` writes the migration Enter confirms. It answered every confirmation with "Migration Cancelled." and wrote nothing, since it read the TUI's starting state, which Enter never changes, instead of the state the TUI ended with

## [1.1.2] - 2026-10-28

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
