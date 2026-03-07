package diff_test

import (
	"testing"

	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestCrossDatabaseComparison(t *testing.T) {
	// Source: Postgres-like schema
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "int4"},
					{Name: "name", DataType: "character varying"},
					{Name: "created_at", DataType: "timestamp with time zone"},
				},
			},
		},
	}

	// Target: MySQL-like schema
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "int"},
					{Name: "name", DataType: "varchar"},
					{Name: "created_at", DataType: "datetime"},
				},
			},
		},
	}

	// Define a mock normalization function that maps both to standard generic types
	normalize := func(dt string) string {
		switch dt {
		case "int4", "int":
			return "integer"
		case "character varying", "varchar":
			return "varchar"
		case "timestamp with time zone", "datetime":
			return "timestamp"
		default:
			return dt
		}
	}

	// Case 1: Without normalization - should have differences in data types
	engineNoNorm := diff.NewDiffEngine(source, target, nil)
	diffsNoNorm := engineNoNorm.Compare()
	assert.Greater(t, len(diffsNoNorm), 0, "Expected differences when comparing natively without normalization")

	foundTypeDiff := false
	for _, d := range diffsNoNorm {
		if d.Object == types.ObjectColumn && d.Type == types.DiffAlter {
			foundTypeDiff = true
		}
	}
	assert.True(t, foundTypeDiff, "Expected at least one type alteration diff")

	// Case 2: With normalization - should have 0 differences
	engineWithNorm := diff.NewDiffEngine(source, target, normalize)
	diffsWithNorm := engineWithNorm.Compare()
	assert.Equal(t, 0, len(diffsWithNorm), "Expected 0 differences after normalization, got: %v", diffsWithNorm)
}
