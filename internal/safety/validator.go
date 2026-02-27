package safety

import (
	"fmt"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

// Validator validates migrations for safety
type Validator struct {
	protectedObjects []string
	dryRun          bool
}

// NewValidator creates a new safety validator
func NewValidator(protectedObjects []string, dryRun bool) *Validator {
	return &Validator{
		protectedObjects: protectedObjects,
		dryRun:          dryRun,
	}
}

// ValidationResult holds the result of a validation
type ValidationResult struct {
	Valid   bool
	Warnings []string
	Errors  []string
}

// ValidateDiff validates a diff for safety
func (v *Validator) ValidateDiff(diff *types.Diff) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check if object is protected
	for _, protected := range v.protectedObjects {
		if strings.Contains(diff.Name, protected) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("cannot modify protected object: %s", diff.Name))
		}
	}

	// Add warnings for dangerous operations
	switch diff.Type {
	case types.DiffDrop:
		result.Warnings = append(result.Warnings, fmt.Sprintf("DROP operation detected: %s", diff.Name))
	case types.DiffAlter:
		if diff.OldValue != "" && diff.NewValue != "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("ALTER operation detected: %s - %s -> %s", diff.Name, diff.OldValue, diff.NewValue))
		}
	}

	return result
}

// ValidateDiffs validates a collection of diffs
func (v *Validator) ValidateDiffs(diffs []types.Diff) ValidationResult {
	result := ValidationResult{Valid: true}

	for _, diff := range diffs {
		r := v.ValidateDiff(&diff)
		if !r.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, r.Errors...)
		}
		result.Warnings = append(result.Warnings, r.Warnings...)
	}

	return result
}

// IsProtected checks if an object name is protected
func (v *Validator) IsProtected(name string) bool {
	for _, protected := range v.protectedObjects {
		if strings.Contains(name, protected) {
			return true
		}
	}
	return false
}

// SetProtectedObjects updates the list of protected objects
func (v *Validator) SetProtectedObjects(objects []string) {
	v.protectedObjects = objects
}
