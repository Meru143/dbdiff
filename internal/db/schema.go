package db

import (
	"context"

	"github.com/meru143/dbdiff/pkg/types"
)

func Introspect(ctx context.Context, db *DB, schemaName string, ignorePatterns []string) (*types.Schema, error) {
	schema := &types.Schema{
		Tables:    make([]types.Table, 0),
		Sequences: make([]types.Sequence, 0),
	}

	tables, err := getTables(ctx, db, schemaName)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		columns, err := getColumns(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, err
		}

		filteredColumns := filterColumns(columns, ignorePatterns)
		if len(filteredColumns) == 0 {
			continue
		}

		indexes, err := getIndexes(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, err
		}

		constraints, err := getConstraints(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, err
		}

		fks, err := getForeignKeys(ctx, db, schemaName, tableName)
		if err != nil {
			return nil, err
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

	sequences, err := getSequences(ctx, db, schemaName)
	if err != nil {
		return nil, err
	}
	schema.Sequences = sequences

	return schema, nil
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
