package db

import (
	"context"
	"fmt"
	"log"

	"github.com/meru143/dbdiff/pkg/types"
)

// Introspector handles database schema introspection
type Introspector struct {
	db             *DB
	schemaName     string
	ignorePatterns []string
	verbose        bool
}

// NewIntrospector creates a new Introspector
func NewIntrospector(db *DB, schemaName string, ignorePatterns []string, verbose bool) *Introspector {
	return &Introspector{
		db:             db,
		schemaName:     schemaName,
		ignorePatterns: ignorePatterns,
		verbose:        verbose,
	}
}

// Introspect performs full schema introspection
func (i *Introspector) Introspect(ctx context.Context) (*types.Schema, error) {
	schema := &types.Schema{
		Tables:    make([]types.Table, 0),
		Sequences: make([]types.Sequence, 0),
		Types:     make([]types.Type, 0),
	}

	// Get tables
	if i.verbose {
		log.Println("Introspecting tables...")
	}
	tables, err := getTables(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	// Process each table
	for idx, tableName := range tables {
		if i.verbose {
			log.Printf("Processing table %d/%d: %s", idx+1, len(tables), tableName)
		}

		columns, err := getColumns(ctx, i.db, i.schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for %s: %w", tableName, err)
		}

		filteredColumns := filterColumns(columns, i.ignorePatterns)
		if len(filteredColumns) == 0 {
			continue
		}

		indexes, err := getIndexes(ctx, i.db, i.schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for %s: %w", tableName, err)
		}

		constraints, err := getConstraints(ctx, i.db, i.schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get constraints for %s: %w", tableName, err)
		}

		fks, err := getForeignKeys(ctx, i.db, i.schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for %s: %w", tableName, err)
		}

		table := types.Table{
			Name:        tableName,
			Columns:     filteredColumns,
			Indexes:     indexes,
			Constraints: constraints,
			ForeignKeys: fks,
		}

		schema.Tables = append(schema.Tables, table)
	}

	// Get sequences
	if i.verbose {
		log.Println("Introspecting sequences...")
	}
	sequences, err := getSequences(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get sequences: %w", err)
	}
	schema.Sequences = sequences

	// Get custom types
	if i.verbose {
		log.Println("Introspecting custom types...")
	}
	customTypes, err := getCustomTypes(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom types: %w", err)
	}
	schema.Types = customTypes

	if i.verbose {
		log.Printf("Introspection complete: %d tables, %d sequences, %d types",
			len(schema.Tables), len(schema.Sequences), len(schema.Types))
	}

	return schema, nil
}

// Introspect performs full schema introspection (legacy function)
func Introspect(ctx context.Context, db *DB, schemaName string, ignorePatterns []string) (*types.Schema, error) {
	introspector := NewIntrospector(db, schemaName, ignorePatterns, false)
	return introspector.Introspect(ctx)
}

func ListTables(ctx context.Context, db *DB, schemaName string) ([]string, error) {
	return getTables(ctx, db, schemaName)
}

type ServerInfo struct {
	Version  string
	Database string
	User     string
}

func GetServerInfo(ctx context.Context, db *DB) (*ServerInfo, error) {
	var info ServerInfo
	err := db.QueryRow(ctx, "SELECT current_database(), current_user, version()").Scan(
		&info.Database, &info.User, &info.Version,
	)
	return &info, err
}
