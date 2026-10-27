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
	dialect      string
	concurrently bool
	transaction  bool
	variables    int // T-SQL variables declared so far, for unique names
}

// NewSQLGenerator creates a new SQL generator
func NewSQLGenerator(diffs types.DiffList, schemaName string, dialect string) *SQLGenerator {
	return NewSQLGeneratorWithSchema(diffs, schemaName, nil, dialect)
}

// NewSQLGeneratorWithSchema creates a new SQL generator with source schema for full SQL generation.
// The source schema supplies the definitions (column types, defaults, constraints, foreign keys)
// that CREATE and ADD statements need; without it they fall back to a comment.
func NewSQLGeneratorWithSchema(diffs types.DiffList, schemaName string, sourceSchema *types.Schema, dialect string) *SQLGenerator {
	if dialect == "" {
		dialect = "postgresql" // default
	}
	return &SQLGenerator{
		diffs:        diffs,
		sourceSchema: sourceSchema,
		schemaName:   schemaName,
		dialect:      dialect,
		concurrently: true,
		transaction:  true,
	}
}

// SetTransaction controls transaction wrapping
func (g *SQLGenerator) SetTransaction(useTransaction bool) {
	g.transaction = useTransaction
}

// SetConcurrently controls CONCURRENTLY on PostgreSQL index changes. A caller that wraps
// the generated SQL in its own transaction must turn it off.
func (g *SQLGenerator) SetConcurrently(concurrently bool) {
	g.concurrently = concurrently
}

// Generate produces SQL statements
func (g *SQLGenerator) Generate() string {
	var sql []string
	seen := make(map[string]bool)
	for _, step := range g.plan() {
		// Several diffs can need the same statement: a MySQL column whose type and
		// nullability both changed is one MODIFY COLUMN.
		stmt := g.generateStatement(&step.diff)
		if stmt == "" || seen[stmt] {
			continue
		}
		seen[stmt] = true
		sql = append(sql, withDetails(&step.details, stmt))
	}
	if len(sql) == 0 {
		return ""
	}
	sql = append([]string{g.lockTimeoutSQL()}, sql...)

	if !g.transaction {
		return strings.Join(sql, "\n\n")
	}
	if g.dialect != "postgresql" {
		return WrapTransaction(g.dialect, strings.Join(sql, "\n\n"))
	}
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

// WrapTransaction wraps migration SQL in the dialect's transaction statements.
func WrapTransaction(dialect, sql string) string {
	sql = strings.TrimRight(sql, "\n")
	switch dialect {
	case "mysql":
		return "-- MySQL commits each DDL statement implicitly: if a statement fails, the ones before it stay applied.\n" +
			"START TRANSACTION;\n\n" + sql + "\n\nCOMMIT;"
	case "sqlserver":
		// Without XACT_ABORT, SQL Server keeps going after a failed statement and COMMIT keeps the rest.
		return "SET XACT_ABORT ON;\nBEGIN TRANSACTION;\n\n" + sql + "\n\nCOMMIT TRANSACTION;"
	default:
		return "BEGIN;\n\n" + sql + "\n\nCOMMIT;"
	}
}

// lockTimeoutSQL makes DDL give up after 10 seconds instead of queueing behind long
// transactions, and blocking every query that queues behind it in turn.
func (g *SQLGenerator) lockTimeoutSQL() string {
	switch g.dialect {
	case "mysql":
		return "SET SESSION lock_wait_timeout = 10;"
	case "sqlserver":
		return "SET LOCK_TIMEOUT 10000;"
	default:
		return "SET lock_timeout = '10s';"
	}
}

// withDetails adds what changed (the diff's description and old and new values) as comments
// under the statement's first line. Definitions of views, functions and triggers are left
// out: the statement itself carries the new one.
func withDetails(diff *types.Diff, stmt string) string {
	var details []string
	if diff.Description != "" {
		details = append(details, diff.Description)
	}
	switch diff.Object {
	case types.ObjectView, types.ObjectMaterializedView, types.ObjectFunction, types.ObjectTrigger:
	default:
		if diff.Type == types.DiffAlter && diff.OldValue != "" {
			details = append(details, "Old: "+diff.OldValue)
		}
		if diff.Type == types.DiffAlter && diff.NewValue != "" {
			details = append(details, "New: "+diff.NewValue)
		}
	}
	if len(details) == 0 {
		return stmt
	}
	first, rest, _ := strings.Cut(stmt, "\n")
	comment := "-- " + strings.ReplaceAll(strings.Join(details, "\n"), "\n", "\n-- ")
	if rest == "" {
		return first + "\n" + comment
	}
	return first + "\n" + comment + "\n" + rest
}

// step is one statement of the migration: the diff it is generated from, the diff whose
// description and old and new values it shows, and where it runs.
type step struct {
	diff    types.Diff
	details types.Diff
	phase   int
	key     string
}

// phases orders statements so each runs after what it depends on: foreign keys, views,
// constraints and indexes go before the columns and tables they cover, tables and columns
// exist before indexes, constraints and foreign keys are added to them, and functions,
// views, triggers and grants come last. Each entry is {create or change, drop}.
var phases = map[types.DiffObject][2]int{
	types.ObjectForeignKey:       {12, 0},
	types.ObjectGrant:            {16, 1},
	types.ObjectTrigger:          {15, 1},
	types.ObjectView:             {14, 1},
	types.ObjectMaterializedView: {14, 1},
	types.ObjectConstraint:       {11, 2},
	types.ObjectIndex:            {10, 3},
	types.ObjectType:             {4, 9},
	types.ObjectSequence:         {4, 9},
	types.ObjectTable:            {5, 8},
	types.ObjectColumn:           {6, 7},
	types.ObjectFunction:         {13, 9}, // dropped before the types their signatures use
}

// plan puts the diffs in execution order. Recreating an index, constraint or foreign key is
// split into its drop and its add, so the drop runs before the columns it covers change: SQL
// Server refuses to alter or drop an indexed column, and MySQL drops a column's index with it.
func (g *SQLGenerator) plan() []step {
	var steps []step
	add := func(diff, details types.Diff) {
		p := phases[diff.Object]
		phase := p[0]
		if diff.Type == types.DiffDrop {
			phase = p[1]
		}
		steps = append(steps, step{diff: diff, details: details, phase: phase, key: g.orderKey(diff)})
	}
	for _, diff := range g.diffs {
		if diff.Type == types.DiffAlter && g.recreatable(&diff) {
			drop, create := diff, diff
			drop.Type, create.Type = types.DiffDrop, types.DiffAdd
			add(drop, types.Diff{})
			add(create, diff)
			continue
		}
		add(diff, diff)
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].phase != steps[j].phase {
			return steps[i].phase < steps[j].phase
		}
		return steps[i].key < steps[j].key
	})
	return steps
}

// recreatable reports whether an ALTER of an index, constraint or foreign key can be split
// into a drop and an add: only when the add can be written, so nothing is dropped that the
// migration can't put back.
func (g *SQLGenerator) recreatable(diff *types.Diff) bool {
	switch diff.Object {
	case types.ObjectIndex:
		return (diff.Index == nil || !diff.Index.IsPrimary) && g.createIndexSQL(diff) != ""
	case types.ObjectConstraint:
		cons := g.sourceConstraint(diff.TableName, diff.Name)
		return cons != nil && g.constraintDefinition(*cons) != ""
	case types.ObjectForeignKey:
		return g.sourceForeignKey(diff.TableName, diff.Name) != nil
	}
	return false
}

// orderKey orders statements within a phase: by object kind and table, and new columns in
// their source order, so the migrated table lists them as the source does.
func (g *SQLGenerator) orderKey(diff types.Diff) string {
	name := diff.Name
	if diff.Object == types.ObjectColumn && diff.Type == types.DiffAdd {
		if t := g.sourceTable(diff.TableName); t != nil {
			for i, col := range t.Columns {
				if col.Name == diff.Name {
					name = fmt.Sprintf("%06d", i)
				}
			}
		}
	}
	return string(diff.Object) + "\x00" + diff.TableName + "\x00" + name
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
	case types.ObjectFunction:
		return g.generateFunctionDiff(diff)
	case types.ObjectTrigger:
		return g.generateTriggerDiff(diff)
	case types.ObjectGrant:
		return g.generateGrantDiff(diff)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateTableDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		if table := g.sourceTable(diff.Name); table != nil {
			return g.generateFullCreateTable(table, schema)
		}
		return fmt.Sprintf("-- Create table: %s\n-- %s", diff.Name, noDefinition)
	case types.DiffDrop:
		if g.dialect == "postgresql" {
			return fmt.Sprintf("-- Drop table: %s\nDROP TABLE IF EXISTS %s%s CASCADE;", diff.Name, schema, diff.Name)
		}
		// MySQL ignores CASCADE here and SQL Server rejects it.
		return fmt.Sprintf("-- Drop table: %s\nDROP TABLE IF EXISTS %s%s;", diff.Name, schema, diff.Name)
	case types.DiffRename:
		if g.dialect == "mysql" {
			return fmt.Sprintf("-- Rename table: %s -> %s\nRENAME TABLE %s%s TO %s%s;",
				diff.OldValue, diff.NewValue, schema, diff.OldValue, schema, diff.NewValue)
		}
		return fmt.Sprintf("-- Rename table: %s -> %s\nALTER TABLE %s%s RENAME TO %s;",
			diff.OldValue, diff.NewValue, schema, diff.OldValue, diff.NewValue)
	default:
		return ""
	}
}

// noDefinition marks a change the generator can't write because the diff has no source
// schema to take the definition from. Emitting a guess (a column typed text, a constraint
// with no body) would either fail or silently change the schema.
const noDefinition = "definition not available, write this change by hand"

// generateFullCreateTable generates a complete CREATE TABLE statement with columns and constraints
func (g *SQLGenerator) generateFullCreateTable(table *types.Table, schema string) string {
	var parts []string
	for _, col := range table.Columns {
		parts = append(parts, g.columnDefinition(col))
	}
	for _, cons := range table.Constraints {
		if def := g.constraintDefinition(cons); def != "" {
			parts = append(parts, def)
		}
	}
	// Foreign keys are added once every new table exists (their own diffs), so tables that
	// reference each other can be created in any order.
	return fmt.Sprintf("-- Create table: %s\nCREATE TABLE %s%s (\n  %s\n);",
		table.Name, schema, table.Name, strings.Join(parts, ",\n  "))
}

// columnDefinition spells out a column as CREATE TABLE, ADD COLUMN and MySQL's MODIFY COLUMN
// take it: name, full type, default, the dialect's extras (identity, auto-increment) and
// nullability, which SQL Server and MySQL would otherwise reset.
func (g *SQLGenerator) columnDefinition(col types.Column) string {
	var sb strings.Builder
	sb.WriteString(col.Name + " " + col.DataType)
	if col.DefaultValue != nil && *col.DefaultValue != "" {
		sb.WriteString(" DEFAULT " + *col.DefaultValue)
	}
	if col.Extra != "" {
		sb.WriteString(" " + col.Extra)
	}
	if col.IsNullable {
		sb.WriteString(" NULL")
	} else {
		sb.WriteString(" NOT NULL")
	}
	return sb.String()
}

// constraintDefinition returns a table constraint clause, e.g. CONSTRAINT uq_email UNIQUE (email),
// or "" for a CHECK whose expression was not introspected.
func (g *SQLGenerator) constraintDefinition(cons types.Constraint) string {
	def := cons.Definition
	if def == "" {
		if cons.Type != "PRIMARY KEY" && cons.Type != "UNIQUE" {
			return ""
		}
		def = fmt.Sprintf("%s (%s)", cons.Type, strings.Join(cons.Columns, ", "))
	}
	if g.dialect == "mysql" && cons.Type == "PRIMARY KEY" {
		return def // MySQL calls every primary key PRIMARY and takes no name for it
	}
	return fmt.Sprintf("CONSTRAINT %s %s", cons.Name, def)
}

// foreignKeyDefinition returns a foreign key clause for CREATE TABLE or ADD.
func (g *SQLGenerator) foreignKeyDefinition(fk types.ForeignKey) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s%s (%s)",
		fk.Name,
		strings.Join(fk.Columns, ", "),
		schemaPrefix(g.schemaName),
		fk.RefTable,
		strings.Join(fk.RefColumns, ", "))

	if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
		fmt.Fprintf(&sb, " ON DELETE %s", fk.OnDelete)
	}
	if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
		fmt.Fprintf(&sb, " ON UPDATE %s", fk.OnUpdate)
	}

	return sb.String()
}

func (g *SQLGenerator) generateColumnDiff(diff *types.Diff) string {
	table := schemaPrefix(g.schemaName) + diff.TableName
	switch diff.Type {
	case types.DiffAdd:
		head := fmt.Sprintf("-- Add column: %s to %s\n", diff.Name, diff.TableName)
		var def string
		if col := g.sourceColumn(diff.TableName, diff.Name); col != nil {
			def = g.columnDefinition(*col)
		} else if diff.NewValue != "" {
			def = diff.Name + " " + diff.NewValue
		} else {
			return head + "-- " + noDefinition
		}
		add := "ADD COLUMN"
		if g.dialect == "sqlserver" {
			add = "ADD" // T-SQL has no COLUMN keyword here
		}
		return fmt.Sprintf("%sALTER TABLE %s %s %s;", head, table, add, def)
	case types.DiffDrop:
		head := fmt.Sprintf("-- Drop column: %s from %s\n", diff.Name, diff.TableName)
		switch g.dialect {
		case "mysql":
			return fmt.Sprintf("%sALTER TABLE %s DROP COLUMN %s;", head, table, diff.Name)
		case "sqlserver":
			// SQL Server refuses to drop a column that still has a DEFAULT constraint.
			return fmt.Sprintf("%s%s\nALTER TABLE %s DROP COLUMN %s;", head, g.dropDefaultSQL(table, diff.Name), table, diff.Name)
		default:
			return fmt.Sprintf("%sALTER TABLE %s DROP COLUMN IF EXISTS %s CASCADE;", head, table, diff.Name)
		}
	case types.DiffAlter:
		return g.alterColumnSQL(diff, table)
	default:
		return ""
	}
}

// Column ALTER diffs say which attribute changed in their description.
const (
	columnTypeChanged     = "Type changed"
	columnNullableChanged = "Nullability changed"
	columnDefaultChanged  = "Default changed"
	columnOrderChanged    = "Column order changed"
)

// columnChange tells which attribute a column ALTER diff changes. Diffs built without a
// description (JSON written by older versions, hand-made diffs) are read from their values.
func columnChange(diff *types.Diff) string {
	switch diff.Description {
	case columnTypeChanged, columnNullableChanged, columnDefaultChanged, columnOrderChanged:
		return diff.Description
	}
	if diff.Name == "__column_order__" {
		return columnOrderChanged
	}
	if diff.OldValue == "NULL" || diff.OldValue == "NOT NULL" {
		return columnNullableChanged
	}
	return columnTypeChanged
}

func (g *SQLGenerator) alterColumnSQL(diff *types.Diff, table string) string {
	head := fmt.Sprintf("-- Alter column: %s.%s\n", diff.TableName, diff.Name)
	change := columnChange(diff)
	if change == columnOrderChanged {
		return fmt.Sprintf("-- Column order of %s differs; this migration does not reorder columns", diff.TableName)
	}
	if change == columnTypeChanged && diff.NewValue == "" {
		return ""
	}
	col := g.sourceColumn(diff.TableName, diff.Name)

	switch g.dialect {
	case "mysql":
		// MODIFY COLUMN replaces the whole definition: anything it leaves out (NOT NULL, the
		// default, AUTO_INCREMENT) is dropped, so it needs the source column.
		if col != nil {
			return fmt.Sprintf("%sALTER TABLE %s MODIFY COLUMN %s;", head, table, g.columnDefinition(*col))
		}
		if change == columnTypeChanged {
			return fmt.Sprintf("%sALTER TABLE %s MODIFY COLUMN %s %s;", head, table, diff.Name, diff.NewValue)
		}
		return head + "-- " + noDefinition
	case "sqlserver":
		if change == columnDefaultChanged {
			stmt := head + g.dropDefaultSQL(table, diff.Name)
			if diff.NewValue != "" {
				stmt += fmt.Sprintf("\nALTER TABLE %s ADD DEFAULT %s FOR %s;", table, diff.NewValue, diff.Name)
			}
			return stmt
		}
		if col == nil {
			if change == columnTypeChanged {
				return fmt.Sprintf("%sALTER TABLE %s ALTER COLUMN %s %s;", head, table, diff.Name, diff.NewValue)
			}
			return head + "-- " + noDefinition
		}
		// ALTER COLUMN resets the column to NULL unless told otherwise, and a type change
		// fails while a DEFAULT constraint depends on the column.
		null := "NULL"
		if !col.IsNullable {
			null = "NOT NULL"
		}
		alter := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s %s;", table, col.Name, col.DataType, null)
		if change != columnTypeChanged {
			return head + alter
		}
		stmt := head + g.dropDefaultSQL(table, col.Name) + "\n" + alter
		if col.DefaultValue != nil && *col.DefaultValue != "" {
			stmt += fmt.Sprintf("\nALTER TABLE %s ADD DEFAULT %s FOR %s;", table, *col.DefaultValue, col.Name)
		}
		return stmt
	default:
		switch change {
		case columnNullableChanged:
			nullable := "DROP NOT NULL"
			if diff.NewValue == "NOT NULL" {
				nullable = "SET NOT NULL"
			}
			return fmt.Sprintf("%sALTER TABLE %s ALTER COLUMN %s %s;", head, table, diff.Name, nullable)
		case columnDefaultChanged:
			if diff.NewValue == "" {
				return fmt.Sprintf("%sALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", head, table, diff.Name)
			}
			return fmt.Sprintf("%sALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", head, table, diff.Name, diff.NewValue)
		default:
			alter := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING (%s::%s);",
				table, diff.Name, diff.NewValue, diff.Name, diff.NewValue)
			if col == nil || !g.targetHasDefault(diff, col) {
				// Nothing to drop. An identity column has no default, and DROP DEFAULT fails on one.
				return head + alter
			}
			// The old default may not cast to the new type ('active' to integer), so it goes
			// first and the source's default comes back after.
			stmt := fmt.Sprintf("%sALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;\n%s", head, table, diff.Name, alter)
			if col.DefaultValue != nil && *col.DefaultValue != "" {
				stmt += fmt.Sprintf("\nALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", table, diff.Name, *col.DefaultValue)
			}
			return stmt
		}
	}
}

// dropDefaultSQL drops a SQL Server column's DEFAULT constraint. The server names it
// (DF__users__statu__3B75D760), so the name is looked up when the migration runs; a column
// without a default leaves the variable NULL, and EXEC of NULL does nothing.
func (g *SQLGenerator) dropDefaultSQL(table, column string) string {
	g.variables++
	v := fmt.Sprintf("@drop_default_%d", g.variables)
	return fmt.Sprintf("DECLARE %[1]s nvarchar(max) = (SELECT N'ALTER TABLE %[2]s DROP CONSTRAINT ' + QUOTENAME(name)"+
		" FROM sys.default_constraints WHERE parent_object_id = OBJECT_ID(N'%[2]s')"+
		" AND parent_column_id = COLUMNPROPERTY(OBJECT_ID(N'%[2]s'), N'%[3]s', 'ColumnId'));\nEXEC (%[1]s);",
		v, table, column)
}

// targetHasDefault reports whether a column has a default on the target: the old value of the
// column's default change when the migration has one, or else the source's default.
func (g *SQLGenerator) targetHasDefault(column *types.Diff, source *types.Column) bool {
	for i := range g.diffs {
		d := &g.diffs[i]
		if d.Object == types.ObjectColumn && d.Type == types.DiffAlter && d.TableName == column.TableName &&
			d.Name == column.Name && columnChange(d) == columnDefaultChanged {
			return d.OldValue != ""
		}
	}
	return source.DefaultValue != nil && *source.DefaultValue != ""
}

// sourceTable finds a table of the source (desired) schema.
func (g *SQLGenerator) sourceTable(name string) *types.Table {
	if g.sourceSchema == nil {
		return nil
	}
	for i := range g.sourceSchema.Tables {
		if g.sourceSchema.Tables[i].Name == name {
			return &g.sourceSchema.Tables[i]
		}
	}
	return nil
}

func (g *SQLGenerator) sourceColumn(table, name string) *types.Column {
	if t := g.sourceTable(table); t != nil {
		for i := range t.Columns {
			if t.Columns[i].Name == name {
				return &t.Columns[i]
			}
		}
	}
	return nil
}

func (g *SQLGenerator) sourceConstraint(table, name string) *types.Constraint {
	if t := g.sourceTable(table); t != nil {
		for i := range t.Constraints {
			if t.Constraints[i].Name == name {
				return &t.Constraints[i]
			}
		}
	}
	return nil
}

func (g *SQLGenerator) sourceForeignKey(table, name string) *types.ForeignKey {
	if t := g.sourceTable(table); t != nil {
		for i := range t.ForeignKeys {
			if t.ForeignKeys[i].Name == name {
				return &t.ForeignKeys[i]
			}
		}
	}
	return nil
}

func (g *SQLGenerator) generateIndexDiff(diff *types.Diff) string {
	if diff.Index != nil && diff.Index.IsPrimary {
		// A primary key's index belongs to its constraint: DROP INDEX fails on it, and
		// CREATE INDEX would not make a primary key.
		return fmt.Sprintf("-- Index %s backs the primary key of %s: change it through the PRIMARY KEY constraint",
			diff.Name, diff.TableName)
	}
	switch diff.Type {
	case types.DiffAdd, types.DiffAlter:
		create := g.createIndexSQL(diff)
		if create == "" {
			// Never drop an index we can't rebuild.
			return fmt.Sprintf("-- Index %s on %s: definition not available, write this change by hand",
				diff.Name, diff.TableName)
		}
		if diff.Type == types.DiffAdd {
			return fmt.Sprintf("-- Create index: %s\n%s", diff.Name, create)
		}
		return fmt.Sprintf("-- Recreate index: %s\n%s\n%s", diff.Name, g.dropIndexSQL(diff), create)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop index: %s\n%s", diff.Name, g.dropIndexSQL(diff))
	default:
		return ""
	}
}

// createIndexSQL returns CREATE INDEX for the index the diff carries, or "" without one. It
// reuses the server's own definition, which keeps what the column list loses (PostgreSQL's
// access method and predicate, MySQL's prefix lengths and FULLTEXT, SQL Server's INCLUDE and
// filter), and builds one from the columns otherwise.
func (g *SQLGenerator) createIndexSQL(diff *types.Diff) string {
	idx := diff.Index
	if idx == nil {
		return ""
	}
	postgres := g.dialect == "postgresql"
	if idx.Definition != "" {
		def := strings.TrimRight(idx.Definition, " \t\r\n;")
		if postgres {
			// pg_get_indexdef reads CREATE [UNIQUE] INDEX name ON ..., so the first "INDEX " is the keyword.
			def = strings.Replace(def, "INDEX ", "INDEX "+g.concurrentlyClause(), 1)
		}
		return def + ";"
	}
	if len(idx.Columns) == 0 {
		return ""
	}
	unique := ""
	if idx.IsUnique {
		unique = "UNIQUE "
	}
	concurrently := ""
	if postgres {
		concurrently = g.concurrentlyClause()
	}
	return fmt.Sprintf("CREATE %sINDEX %s%s ON %s%s (%s);", unique, concurrently, diff.Name,
		schemaPrefix(g.schemaName), diff.TableName, strings.Join(idx.Columns, ", "))
}

// dropIndexSQL returns DROP INDEX. PostgreSQL index names are schema-scoped; MySQL and
// SQL Server name the table instead.
func (g *SQLGenerator) dropIndexSQL(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	if g.dialect == "mysql" || g.dialect == "sqlserver" {
		return fmt.Sprintf("DROP INDEX %s ON %s%s;", diff.Name, schema, diff.TableName)
	}
	return fmt.Sprintf("DROP INDEX %sIF EXISTS %s%s;", g.concurrentlyClause(), schema, diff.Name)
}

// concurrentlyClause returns PostgreSQL's CONCURRENTLY, which builds and drops indexes
// without blocking writes but cannot run inside a transaction block or a DO block, so it
// is only used when neither this generator nor its caller wraps the migration in one.
func (g *SQLGenerator) concurrentlyClause() string {
	if g.concurrently && !g.transaction {
		return "CONCURRENTLY "
	}
	return ""
}

func (g *SQLGenerator) generateConstraintDiff(diff *types.Diff) string {
	table := schemaPrefix(g.schemaName) + diff.TableName
	var add string
	if cons := g.sourceConstraint(diff.TableName, diff.Name); cons != nil {
		if def := g.constraintDefinition(*cons); def != "" {
			add = fmt.Sprintf("ALTER TABLE %s ADD %s;", table, def)
		}
	}
	switch diff.Type {
	case types.DiffAdd:
		if add == "" {
			return fmt.Sprintf("-- Add constraint: %s\n-- %s", diff.Name, noDefinition)
		}
		return fmt.Sprintf("-- Add constraint: %s\n%s", diff.Name, add)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop constraint: %s\n%s", diff.Name, g.dropConstraintSQL(table, diff.Name))
	case types.DiffAlter:
		// Never drop a constraint we can't add back.
		if add == "" {
			return fmt.Sprintf("-- Alter constraint: %s\n-- %s", diff.Name, noDefinition)
		}
		return fmt.Sprintf("-- Alter constraint: %s\n%s\n%s", diff.Name, g.dropConstraintSQL(table, diff.Name), add)
	default:
		return ""
	}
}

func (g *SQLGenerator) dropConstraintSQL(table, name string) string {
	switch g.dialect {
	case "mysql":
		if name == "PRIMARY" {
			return fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY;", table)
		}
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", table, name)
	case "sqlserver":
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", table, name)
	default:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", table, name)
	}
}

func (g *SQLGenerator) generateForeignKeyDiff(diff *types.Diff) string {
	table := schemaPrefix(g.schemaName) + diff.TableName
	var add string
	if fk := g.sourceForeignKey(diff.TableName, diff.Name); fk != nil {
		add = fmt.Sprintf("ALTER TABLE %s ADD %s;", table, g.foreignKeyDefinition(*fk))
	}
	drop := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", table, diff.Name)
	switch g.dialect {
	case "mysql":
		drop = fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s;", table, diff.Name)
	case "sqlserver":
		drop = fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", table, diff.Name)
	}
	switch diff.Type {
	case types.DiffAdd:
		if add == "" {
			return fmt.Sprintf("-- Add foreign key: %s\n-- %s", diff.Name, noDefinition)
		}
		return fmt.Sprintf("-- Add foreign key: %s\n%s", diff.Name, add)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop foreign key: %s\n%s", diff.Name, drop)
	case types.DiffAlter:
		// Never drop a foreign key we can't add back.
		if add == "" {
			return fmt.Sprintf("-- Alter foreign key: %s\n-- %s", diff.Name, noDefinition)
		}
		return fmt.Sprintf("-- Alter foreign key: %s\n%s\n%s", diff.Name, drop, add)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateSequenceDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		// CREATE SEQUENCE alone makes a bigint sequence; a serial column's is capped at
		// 2147483647, so the source's bounds are spelled out.
		if g.sourceSchema != nil {
			for _, seq := range g.sourceSchema.Sequences {
				if seq.Name == diff.Name {
					cycle := ""
					if seq.Cycle {
						cycle = " CYCLE"
					}
					return fmt.Sprintf("-- Create sequence: %s\nCREATE SEQUENCE %s%s INCREMENT BY %d MINVALUE %d MAXVALUE %d START WITH %d CACHE %d%s;",
						diff.Name, schema, diff.Name, seq.Increment, seq.MinValue, seq.MaxValue, seq.Start, seq.Cache, cycle)
				}
			}
		}
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
		if g.sourceSchema != nil {
			for _, t := range g.sourceSchema.Types {
				if t.Name == diff.Name && t.Kind == "enum" {
					labels := make([]string, len(t.Values))
					for i, v := range t.Values {
						labels[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
					}
					return fmt.Sprintf("-- Create type: %s\nCREATE TYPE %s%s AS ENUM (%s);",
						diff.Name, schema, diff.Name, strings.Join(labels, ", "))
				}
			}
		}
		// Composite types are not introspected beyond their name.
		return fmt.Sprintf("-- Create type: %s\n-- %s", diff.Name, noDefinition)
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

func (g *SQLGenerator) generateFunctionDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	definition := strings.TrimRight(diff.NewValue, " \t\r\n;")
	// Notice: diff.Name contains func_name(args) to handle overloads specifically for drop
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create function: %s\n%s;",
			diff.Name, definition)
	case types.DiffAlter:
		return fmt.Sprintf("-- Alter function: %s\n%s;",
			diff.Name, definition)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop function: %s\nDROP FUNCTION IF EXISTS %s%s CASCADE;",
			diff.Name, schema, diff.Name)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateTriggerDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	definition := strings.TrimRight(diff.NewValue, " \t\r\n;")
	switch diff.Type {
	case types.DiffAdd:
		return fmt.Sprintf("-- Create trigger: %s on %s\n%s;",
			diff.Name, diff.TableName, definition)
	case types.DiffAlter:
		return fmt.Sprintf("-- Alter trigger: %s on %s\nDROP TRIGGER IF EXISTS %s ON %s%s CASCADE;\n%s;",
			diff.Name, diff.TableName, diff.Name, schema, diff.TableName, definition)
	case types.DiffDrop:
		return fmt.Sprintf("-- Drop trigger: %s on %s\nDROP TRIGGER IF EXISTS %s ON %s%s CASCADE;",
			diff.Name, diff.TableName, diff.Name, schema, diff.TableName)
	default:
		return ""
	}
}

func (g *SQLGenerator) generateGrantDiff(diff *types.Diff) string {
	schema := schemaPrefix(g.schemaName)
	switch diff.Type {
	case types.DiffAdd:
		grantable := ""
		if diff.OldValue == "true" {
			grantable = " WITH GRANT OPTION"
		}
		return fmt.Sprintf("-- Grant %s on table %s to %s\nGRANT %s ON TABLE %s%s TO %s%s;",
			diff.Name, diff.TableName, diff.NewValue, diff.Name, schema, diff.TableName, diff.NewValue, grantable)
	case types.DiffDrop:
		return fmt.Sprintf("-- Revoke %s on table %s from %s\nREVOKE %s ON TABLE %s%s FROM %s CASCADE;",
			diff.Name, diff.TableName, diff.OldValue, diff.Name, schema, diff.TableName, diff.OldValue)
	default:
		return ""
	}
}

func GenerateSQL(diffs types.DiffList, schemaName string, dialect string) string {
	gen := NewSQLGenerator(diffs, schemaName, dialect)
	return gen.Generate()
}
