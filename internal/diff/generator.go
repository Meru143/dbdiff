package diff

import (
	"fmt"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

// SQLGenerator generates SQL statements from diffs
type SQLGenerator struct {
	diffs           types.DiffList
	schemaName      string
	lockTimeout     string
	concurrently    bool
	transaction     bool
}

// NewSQLGenerator creates a new SQL generator
func NewSQLGenerator(diffs types.DiffList, schemaName string) *SQLGenerator {
	return &SQLGenerator{
		diffs:      diffs,
		schemaName: schemaName,
		lockTimeout: "10s",
		concurrently: true,
		transaction: true,
	}
}

// Generate produces SQL statements
func (g *SQLGenerator) Generate() string {
	var sql []string

	// Add lock timeout at the start
	if g.lockTimeout != "" {
		sql = append(sql, fmt.Sprintf("SET lock_timeout = '%s';", g.lockTimeout))
	}

	// Sort diffs: CREATE TABLE first, then ALTER, then DROP
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
	// Separate by type
	var creates, alters, drops types.DiffList

	for _, diff := range g.diffs {
		switch diff.Type {
		case types.DiffAdd:
			creates = append(creates, diff)
		case types.DiffAlter:
			alters = append(alters, diff)
		case types.DiffDrop:
			drops = append(drops, diff)
		}
	}

	// Reverse drops so tables are dropped after their dependencies
	for i, j := 0, len(drops)-1; i < j; i, j = i+1, j-1 {
		drops[i], drops[j] = drops[j], drops[i]
	}

	// Order: creates, alters, drops
	var sorted types.DiffList
	sorted = append(sorted, creates...)
	sorted = append(sorted, alters...)
	sorted = append(sorted, drops...)

	return sorted
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
	default:
		return ""
	}
}

func (g *SQLGenerator) generateTableDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create table: %s\nCREATE TABLE %s%s ();", diff.Name, schema, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop table: %s\nDROP TABLE %s%s CASCADE;", diff.Name, schema, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateColumnDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Add column: %s to %s\nALTER TABLE %s%s ADD COLUMN %s;",
			diff.Name, diff.TableName, schema, diff.TableName, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop column: %s from %s\nALTER TABLE %s%s DROP COLUMN %s CASCADE;",
			diff.Name, diff.TableName, schema, diff.TableName, diff.Name)
	case types.DiffAlter:
		if diff.OldValue != "" && diff.NewValue != "" {
			// Type change
			return fmt.Sprintf("-- Alter column: %s.%s\nALTER TABLE %s%s ALTER COLUMN %s TYPE %s;",
				diff.TableName, diff.Name, schema, diff.TableName, diff.Name, diff.NewValue)
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
		return fmt.Sprintf("-- Create index: %s\nCREATE INDEX %sON %s%s ();",
			diff.Name, concurrently, schema, diff.TableName)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop index: %s\nDROP INDEX %s%s;",
			diff.Name, concurrently, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateConstraintDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Add constraint: %s\nALTER TABLE %s%s ADD CONSTRAINT %s;",
			diff.Name, schema, diff.TableName, diff.Name)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop constraint: %s\nALTER TABLE %s%s DROP CONSTRAINT %s;",
			diff.TableName, schema, diff.TableName, diff.Name)
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
		return fmt.Sprintf("-- Drop foreign key: %s\nALTER TABLE %s%s DROP CONSTRAINT %s;",
			diff.TableName, schema, diff.TableName, diff.Name)
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
		return fmt.Sprintf("-- Drop sequence: %s\nDROP SEQUENCE %s%s;",
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
		return fmt.Sprintf("-- Drop type: %s\nDROP TYPE %s%s;",
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

// GenerateSQL is a legacy function
func GenerateSQL(diffs types.DiffList, schemaName string) string {
	gen := NewSQLGenerator(diffs, schemaName)
	return gen.Generate()
}
