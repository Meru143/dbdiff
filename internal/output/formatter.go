package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

type Formatter struct {
	format string
}

func NewFormatter(format string) *Formatter {
	return &Formatter{format: format}
}

func (f *Formatter) Format(diffs types.DiffList) (string, error) {
	switch f.format {
	case "sql":
		return f.formatSQL(diffs), nil
	case "table":
		return f.formatTable(diffs), nil
	case "json":
		return f.formatJSON(diffs), nil
	default:
		return f.formatSQL(diffs), nil
	}
}

func (f *Formatter) FormatMigration(diffs types.DiffList, transaction bool) (string, error) {
	sql := f.formatSQL(diffs)
	if transaction {
		sql = "BEGIN;\n\n" + sql + "\nCOMMIT;"
	}
	return sql, nil
}

func (f *Formatter) formatSQL(diffs types.DiffList) string {
	var sb strings.Builder
	for _, diff := range diffs {
		sb.WriteString(fmt.Sprintf("-- %s %s: %s\n", diff.Type, diff.Object, diff.Name))
		if diff.TableName != "" {
			sb.WriteString(fmt.Sprintf("-- Table: %s\n", diff.TableName))
		}
		if diff.OldValue != "" {
			sb.WriteString(fmt.Sprintf("-- Old: %s\n", diff.OldValue))
		}
		if diff.NewValue != "" {
			sb.WriteString(fmt.Sprintf("-- New: %s\n", diff.NewValue))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (f *Formatter) formatTable(diffs types.DiffList) string {
	var sb strings.Builder
	sb.WriteString("\n+------+--------+--------------+---------------+---------------------------+\n")
	sb.WriteString("| Type | Object | Name         | Table         | Details                   |\n")
	sb.WriteString("+------+--------+--------------+---------------+---------------------------+\n")

	for _, diff := range diffs {
		details := ""
		if diff.OldValue != "" && diff.NewValue != "" {
			details = fmt.Sprintf("%s -> %s", diff.OldValue, diff.NewValue)
		}
		sb.WriteString(fmt.Sprintf("| %-4s | %-6s | %-12s | %-13s | %-25s |\n",
			truncate(string(diff.Type), 4),
			truncate(string(diff.Object), 6),
			truncate(diff.Name, 12),
			truncate(diff.TableName, 13),
			truncate(details, 25),
		))
	}
	sb.WriteString("+------+--------+--------------+---------------+---------------------------+\n")
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-2] + ".."
	}
	return s
}

func (f *Formatter) formatJSON(diffs types.DiffList) string {
	return fmt.Sprintf("%+v", diffs)
}

func WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
