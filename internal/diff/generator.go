package diff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

// SQLGenerator generates SQL statements from diffs
type SQLGenerator struct {
	diffs        types.DiffList
	sourceSchema *types.Schema
	schemaName   string
	lockTimeout  string
	concurrently bool
	transaction  bool
}

// NewSQLGenerator creates a new SQL generator
func NewSQLGenerator(diffs types.DiffList, schemaName string) *SQLGenerator {
	return &SQLGenerator{
		diffs:        diffs,
		sourceSchema: nil,
		schemaName:   schemaName,
		lockTimeout:  "10s",
		concurrently: true,
		transaction:  true,
	}
}

// NewSQLGeneratorWithSchema creates a new SQL generator with source schema for full SQL generation
func NewSQLGeneratorWithSchema(diffs types.DiffList, schemaName string, sourceSchema *types.Schema) *SQLGenerator {
	return &SQLGenerator{
		diffs:        diffs,
		sourceSchema: sourceSchema,
		schemaName:   schemaName,
		lockTimeout:  "10s",
		concurrently: true,
		transaction:  true,
	}
}

// SetTransaction controls transaction wrapping
func (g *SQLGenerator) SetTransaction(useTransaction bool) {
	g.transaction = useTransaction
}

// Generate produces SQL statements
func (g *SQLGenerator) Generate() string {
	var sql []string

	// Add lock timeout at the start
	if g.lockTimeout != "" {
		sql = append(sql, fmt.Sprintf("SET lock_timeout = '%s';", g.lockTimeout))
	}

	// Sort diffs: CREATE TABLE first, then ALTER, then DROP (real topological sort)
	sorted := g.topologicalSort()

	// Generate each statement
	for _, diff := range sorted {
		stmt := g.generateStatement(&diff)
		if stmt != "" {
			sql = append(sql, stmt)
		}
	}

	// Wrap in transaction if requested
	if g.transaction && len(sql) > 0 {
		transactionalSQL := []string{"DO $$"}
		transactionalSQL = append(transactionalSQL, "BEGIN")
		transactionalSQL = append(transactionalSQL, sql...)
		transactionalSQL = append(transactionalSQL, "EXCEPTION WHEN OTHERS THEN")
		transactionalSQL = append(transactionalSQL, "  RAISE NOTICE 'Migration failed: %', SQLERRM;")
		transactionalSQL = append(transactionalSQL, "  ROLLBACK;")
		transactionalSQL = append(transactionalSQL, "  RAISE;")
		transactionalSQL = append(transactionalSQL, "END $$;")
		return strings.Join(transactionalSQL, "\n")
	}

	return strings.Join(sql, "\n")
}

func (g *SQLGenerator) topologicalSort() types.DiffList {
	// Build dependency graph
	dependencies := make(map[string][]string) // diffName -> depends on

	// Separate by type
	var creates, alters, drops types.DiffList

	for _, diff := range g.diffs {
		switch diff.Type {
		case types.DiffAdd:
			// Track table dependencies for FKs
			if diff.Object == types.ObjectForeignKey {
				dependencies[diff.Name] = []string{diff.TableName, diff.NewValue}
			}
			creates = append(creates, diff)
		case types.DiffAlter:
			alters = append(alters, diff)
		case types.DiffDrop:
			// For drops, reverse order
			drops = append([]types.Diff{diff}, drops...)
		case types.DiffRename:
			// Handle renames: drop old, add new
			creates = append(creates, types.Diff{
				Type:   types.DiffAdd,
				Object: diff.Object,
				Name:   diff.NewValue,
			})
			drops = append(drops, types.Diff{
				Type:   types.DiffDrop,
				Object: diff.Object,
				Name:   diff.OldValue,
			})
		}
	}

	// Order creates by dependencies (tables before FKs)
	sort.SliceStable(creates, func(i, j int) bool {
		return !dependsOn(creates[i], creates[j], dependencies)
	})

	// Order alters
	sort.SliceStable(alters, func(i, j int) bool {
		// Columns before constraints
		order := map[types.DiffObject]int{
			types.ObjectColumn:           1,
			types.ObjectIndex:            2,
			types.ObjectConstraint:       3,
			types.ObjectForeignKey:       4,
			types.ObjectSequence:         5,
			types.ObjectType:             6,
			types.ObjectView:             7,
			types.ObjectMaterializedView: 8,
		}
		return order[alters[i].Object] < order[alters[j].Object]
	})

	// Order: creates, alters, drops
	var sorted types.DiffList
	sorted = append(sorted, creates...)
	sorted = append(sorted, alters...)
	sorted = append(sorted, drops...)

	return sorted
}

func dependsOn(a, b types.Diff, deps map[string][]string) bool {
	for _, dep := range deps[a.Name] {
		if dep == b.Name {
			return true
		}
	}
	return false
}

func (g *SQLGenerator) generateStatement(diff *types.Diff) string {
	switch diff.Object {
	case types.ObjectTable:
		return g.generateTableDiff(diff)
	case types.ObjectColumn:
		return g.generateColumnDiff(diff)
	case types.ObjectIndex:
		return g.generateIndexDiff(diff)
	case types.ObjectConstraint:
		return g.generateConstraintDiff(diff)
	case types.ObjectForeignKey:
		return g.generateForeignKeyDiff(diff)
	case types.ObjectSequence:
		return g.generateSequenceDiff(diff)
	case types.ObjectType:
		return g.generateTypeDiff(diff)
	case types.ObjectView:
		return g.generateViewDiff(diff)
	case types.ObjectMaterializedView:
		return g.generateMaterializedViewDiff(diff)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateTableDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		// Try to get full table definition from source schema
		if g.sourceSchema != nil {
			for _, table := range g.sourceSchema.Tables {
				if table.Name == diff.Name {
					return g.generateFullCreateTable(&table, schema)
				}
			}
		}
		// Fallback if no source schema
		return fmt.Sprintf("-- Create table: %s\nCREATE TABLE IF NOT EXISTS %s%s ();", diff.Name, schema, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop table: %s\nDROP TABLE IF EXISTS %s%s CASCADE;", diff.Name, schema, diff.Name)
	case types.DiffRename:
		return fmt.Sprintf("-- Rename table: %s -> %s\nALTER TABLE %s%s RENAME TO %s;",
			diff.OldValue, diff.NewValue, schema, diff.OldValue, diff.NewValue)
	default:
		return ""
	}
}

// generateFullCreateTable generates a complete CREATE TABLE statement with columns and constraints
func (g *SQLGenerator) generateFullCreateTable(table *types.Table, schema string) string {
	var parts []string

	// Generate column definitions
	var colDefs []string
	for _, col := range table.Columns {
		colDef := g.generateColumnDefinition(col)
		colDefs = append(colDefs, colDef)
	}

	// Generate table-level constraints (UNIQUE, CHECK)
	for _, cons := range table.Constraints {
		if cons.Type == "UNIQUE" || cons.Type == "CHECK" {
			colDefs = append(colDefs, g.generateConstraintDefinition(cons, table.Name))
		}
	}

	if len(colDefs) > 0 {
		parts = append(parts, strings.Join(colDefs, ",\n  "))
	}

	// Generate primary key
	for _, cons := range table.Constraints {
		if cons.Type == "PRIMARY KEY" {
			parts = append(parts, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(cons.Columns, ", ")))
		}
	}

	// Generate foreign keys
	for _, fk := range table.ForeignKeys {
		parts = append(parts, g.generateForeignKeyDefinition(fk))
	}

	// Build the CREATE TABLE statement
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- Create table: %s\n", table.Name))
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s%s (\n  %s\n);", schema, table.Name, strings.Join(parts, ",\n  ")))

	return sb.String()
}

// generateColumnDefinition generates a column definition
func (g *SQLGenerator) generateColumnDefinition(col types.Column) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s", col.Name, col.DataType))

	// Add default value
	if col.DefaultValue != nil && *col.DefaultValue != "" {
		sb.WriteString(fmt.Sprintf(" DEFAULT %s", *col.DefaultValue))
	}

	// Add null constraint
	if !col.IsNullable {
		sb.WriteString(" NOT NULL")
	}

	return sb.String()
}

// generateConstraintDefinition generates a table-level constraint definition
func (g *SQLGenerator) generateConstraintDefinition(cons types.Constraint, tableName string) string {
	switch cons.Type {
	case "UNIQUE":
		return fmt.Sprintf("UNIQUE (%s)", strings.Join(cons.Columns, ", "))
	case "CHECK":
		// Check constraints need a definition - use placeholder if not available
		return fmt.Sprintf("CONSTRAINT %s CHECK (%s)", cons.Name, strings.Join(cons.Columns, " AND "))
	default:
		return fmt.Sprintf("CONSTRAINT %s", cons.Name)
	}
}

// generateForeignKeyDefinition generates a foreign key constraint
func (g *SQLGenerator) generateForeignKeyDefinition(fk types.ForeignKey) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
		fk.Name,
		strings.Join(fk.Columns, ", "),
		fk.RefTable,
		strings.Join(fk.RefColumns, ", ")))

	if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
		sb.WriteString(fmt.Sprintf(" ON DELETE %s", fk.OnDelete))
	}
	if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
		sb.WriteString(fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate))
	}

	return sb.String()
}

func (g *SQLGenerator) generateColumnDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		// NewValue contains the data type from comparison
		dataType := diff.NewValue
		if dataType == "" {
			dataType = "text" // fallback
		}
		return fmt.Sprintf("-- Add column: %s to %s\nALTER TABLE %s%s ADD COLUMN %s %s;",
			diff.Name, diff.TableName, schema, diff.TableName, diff.Name, dataType)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop column: %s from %s\nALTER TABLE %s%s DROP COLUMN IF EXISTS %s CASCADE;",
			diff.Name, diff.TableName, schema, diff.TableName, diff.Name)
	case types.DiffAlter:
		if diff.OldValue != "" && diff.NewValue != "" {
			// Check if it's a type change or nullable change
			if diff.OldValue == "NULL" || diff.OldValue == "NOT NULL" {
				// Nullable change
				nullable := "DROP NOT NULL"
				if diff.NewValue == "NOT NULL" {
					nullable = "SET NOT NULL"
				}
				return fmt.Sprintf("-- Alter column: %s.%s\nALTER TABLE %s%s ALTER COLUMN %s %s;",
					diff.TableName, diff.Name, schema, diff.TableName, diff.Name, nullable)
			}
			// Type change with USING clause
			return fmt.Sprintf("-- Alter column: %s.%s\nALTER TABLE %s%s ALTER COLUMN %s TYPE %s USING (%s::%s);",
				diff.TableName, diff.Name, schema, diff.TableName, diff.Name, diff.NewValue, diff.Name, diff.NewValue)
		}
		return ""
	default:
		return ""
	}
}

func (g *SQLGenerator) generateIndexDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	concurrently := ""
	if g.concurrently {
		concurrently = "CONCURRENTLY "
	}
	switch diff.Type {
	case types.DiffAdd:
		// Note: Full index creation would need column list from source
		return fmt.Sprintf("-- Create index: %s\nCREATE INDEX %sON %s%s ();",
			diff.Name, concurrently, schema, diff.TableName)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop index: %s\nDROP INDEX IF EXISTS %s;",
			concurrently, diff.Name)
	case types.DiffAlter:
		// Recreate index for definition changes
		return fmt.Sprintf("-- Recreate index: %s\nDROP INDEX IF EXISTS %s;\nCREATE INDEX %sON %s%s ();",
			diff.Name, diff.Name, concurrently, schema, diff.TableName)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateConstraintDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		// Note: Full constraint would need constraint type and definition
		return fmt.Sprintf("-- Add constraint: %s\nALTER TABLE %s%s ADD CONSTRAINT %s;",
			diff.Name, schema, diff.TableName, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop constraint: %s\nALTER TABLE %s%s DROP CONSTRAINT IF EXISTS %s;",
			diff.Name, schema, diff.TableName, diff.Name)
	case types.DiffAlter:
		// Drop and recreate
		return fmt.Sprintf("-- Alter constraint: %s\nALTER TABLE %s%s DROP CONSTRAINT IF EXISTS %s;"+"\n"+"ALTER TABLE %s%s ADD CONSTRAINT %s;",
			diff.Name, schema, diff.TableName, diff.Name, schema, diff.TableName, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateForeignKeyDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Add foreign key: %s\nALTER TABLE %s%s ADD CONSTRAINT %s;",
			diff.Name, schema, diff.TableName, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop foreign key: %s\nALTER TABLE %s%s DROP CONSTRAINT IF EXISTS %s;",
			diff.Name, schema, diff.TableName, diff.Name)
	case types.DiffAlter:
		// For FK changes, drop and recreate
		// Extract action from Description if available
		return fmt.Sprintf("-- Alter foreign key: %s\nALTER TABLE %s%s DROP CONSTRAINT IF EXISTS %s;"+"\n"+"ALTER TABLE %s%s ADD CONSTRAINT %s;",
			diff.Name, schema, diff.TableName, diff.Name, schema, diff.TableName, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateSequenceDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create sequence: %s\nCREATE SEQUENCE %s%s;",
			diff.Name, schema, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop sequence: %s\nDROP SEQUENCE IF EXISTS %s%s;",
			diff.Name, schema, diff.Name)
	case types.DiffAlter:
		if diff.NewValue != "" {
			return fmt.Sprintf("-- Alter sequence: %s\nALTER SEQUENCE %s%s %s;",
				diff.Name, schema, diff.Name, diff.NewValue)
		}
		return ""
	default:
		return ""
	}
}

func (g *SQLGenerator) generateTypeDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create type: %s\nCREATE TYPE %s%s;",
			diff.Name, schema, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop type: %s\nDROP TYPE IF EXISTS %s%s;",
			diff.Name, schema, diff.Name)
	default:
		return ""
	}
}

func schemaPrefix(schema string) string {
	if schema == "" || schema == "public" {
		return ""
	}
	return schema + "."
}

func (g *SQLGenerator) generateViewDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	definition := strings.TrimRight(diff.NewValue, " \t\r\n;")
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create view: %s\nCREATE OR REPLACE VIEW %s%s AS\n%s;",
			diff.Name, schema, diff.Name, definition)
	case types.DiffAlter:
		return fmt.Sprintf("-- Alter view: %s\nCREATE OR REPLACE VIEW %s%s AS\n%s;",
			diff.Name, schema, diff.Name, definition)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop view: %s\nDROP VIEW IF EXISTS %s%s CASCADE;",
			diff.Name, schema, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateMaterializedViewDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	definition := strings.TrimRight(diff.NewValue, " \t\r\n;")
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create materialized view: %s\nCREATE MATERIALIZED VIEW %s%s AS\n%s;",
			diff.Name, schema, diff.Name, definition)
	case types.DiffAlter:
		// Drop and recreate for materialized views since CREATE OR REPLACE is not supported for them in PostgreSQL
		return fmt.Sprintf("-- Alter materialized view: %s\nDROP MATERIALIZED VIEW IF EXISTS %s%s CASCADE;\nCREATE MATERIALIZED VIEW %s%s AS\n%s;",
			diff.Name, schema, diff.Name, schema, diff.Name, definition)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop materialized view: %s\nDROP MATERIALIZED VIEW IF EXISTS %s%s CASCADE;",
			diff.Name, schema, diff.Name)
	default:
		return ""
	}
}

// GenerateSQL is a legacy function
func GenerateSQL(diffs types.DiffList, schemaName string) string {
	gen := NewSQLGenerator(diffs, schemaName)
	return gen.Generate()
}
