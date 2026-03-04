package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meru143/dbdiff/pkg/types"
)

func TestInitialModel(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}

	m := InitialModel(diffs)

	if len(m.diffs) != 2 {
		t.Errorf("Expected 2 diffs, got %d", len(m.diffs))
	}

	// All should be selected by default
	if len(m.selected) != 2 {
		t.Errorf("Expected 2 selected items, got %d", len(m.selected))
	}

	selectedDiffs := m.GetSelectedDiffs()
	if len(selectedDiffs) != 2 {
		t.Errorf("Expected 2 selected diffs returned, got %d", len(selectedDiffs))
	}
}

func TestModelSelectionToggle(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}

	m := InitialModel(diffs)

	// Simulate pressing space on the first item
	m.cursor = 0
	msg := tea.KeyMsg{Type: tea.KeySpace}

	newModel, _ := m.Update(msg)
	updatedM := newModel.(Model)

	// Item 0 should now be deselected
	if _, ok := updatedM.selected[0]; ok {
		t.Error("Expected item 0 to be deselected")
	}

	// Only 1 item should be selected now
	selectedDiffs := updatedM.GetSelectedDiffs()
	if len(selectedDiffs) != 1 {
		t.Errorf("Expected 1 selected diff returned, got %d", len(selectedDiffs))
	}
	if selectedDiffs[0].Name != "email" {
		t.Errorf("Expected 'email' to be the only selected diff, got %s", selectedDiffs[0].Name)
	}
}
