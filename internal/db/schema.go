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
		Tables:            make([]types.Table, 0),
		Sequences:         make([]types.Sequence, 0),
		Types:             make([]types.Type, 0),
		Views:             make([]types.View, 0),
		MaterializedViews: make([]types.MaterializedView, 0),
		Functions:         make([]types.Function, 0),
		Triggers:          make([]types.Trigger, 0),
		Grants:            make([]types.Grant, 0),
	}

	// Get tables
	if i.verbose {
		log.Println("Introspecting tables...")
	}
	tables, err := getTables(ctx, i.db, i.schemaName, i.ignorePatterns)
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

	// Get views
	if i.verbose {
		log.Println("Introspecting views...")
	}
	views, err := getViews(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get views: %w", err)
	}
	schema.Views = views

	// Get materialized views
	if i.verbose {
		log.Println("Introspecting materialized views...")
	}
	matViews, err := getMaterializedViews(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get materialized views: %w", err)
	}
	schema.MaterializedViews = matViews

	// Get functions
	if i.verbose {
		log.Println("Introspecting functions...")
	}
	functions, err := getFunctions(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get functions: %w", err)
	}
	schema.Functions = functions

	// Get triggers
	if i.verbose {
		log.Println("Introspecting triggers...")
	}
	triggers, err := getTriggers(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get triggers: %w", err)
	}
	schema.Triggers = triggers

	// Get grants
	if i.verbose {
		log.Println("Introspecting grants...")
	}
	grants, err := getGrants(ctx, i.db, i.schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get grants: %w", err)
	}
	schema.Grants = grants

	if i.verbose {
		log.Printf("Introspection complete: %d tables, %d sequences, %d types, %d views, %d materialized views, %d functions, %d triggers, %d grants",
			len(schema.Tables), len(schema.Sequences), len(schema.Types), len(schema.Views), len(schema.MaterializedViews), len(schema.Functions), len(schema.Triggers), len(schema.Grants))
	}

	return schema, nil
}
