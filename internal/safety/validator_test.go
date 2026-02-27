package safety

import (
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

func TestValidator_NewValidator(t *testing.T) {
	protected := []string{"users", "accounts"}
	v := NewValidator(protected, false)

	if v == nil {
		t.Fatal("Expected validator, got nil")
	}
}

func TestValidator_IsProtected(t *testing.T) {
	protected := []string{"users", "accounts"}
	v := NewValidator(protected, false)

	tests := []struct {
		name     string
		objName  string
		expected bool
	}{
		{"users table", "users", true},
		{"accounts table", "accounts", true},
		{"other table", "posts", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.IsProtected(tt.objName)
			if result != tt.expected {
				t.Errorf("IsProtected(%s) = %v, want %v", tt.objName, result, tt.expected)
			}
		})
	}
}

func TestValidator_ValidateDiff(t *testing.T) {
	protected := []string{"users"}
	v := NewValidator(protected, false)

	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffDrop, Object: types.ObjectTable, Name: "posts"},
	}

	result := v.ValidateDiff(&diffs[0])
	if len(result.Errors) == 0 {
		t.Error("Expected error for protected table 'users'")
	}

	result = v.ValidateDiff(&diffs[1])
	if len(result.Errors) > 0 {
		t.Error("Expected no error for non-protected table 'posts'")
	}
}

func TestValidator_SetProtectedObjects(t *testing.T) {
	v := NewValidator([]string{}, false)
	v.SetProtectedObjects([]string{"admin", "config"})

	if !v.IsProtected("admin") {
		t.Error("Expected 'admin' to be protected")
	}
	if v.IsProtected("users") {
		t.Error("Expected 'users' not to be protected")
	}
}
