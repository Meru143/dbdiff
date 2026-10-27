package tui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/meru143/dbdiff/pkg/types"
)

func TestInitialModel(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}

	m := InitialModel(diffs, output.FormatOptions{Format: "sql"})

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

	m := InitialModel(diffs, output.FormatOptions{Format: "sql"})

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

// Enter confirms in the model the program ends with. Update takes the model by value, so
// the one the program started from stays unconfirmed: interactive read that one, and
// answered every confirmation with "Migration Cancelled.".
func TestEnterConfirmsFinalModel(t *testing.T) {
	diffs := types.DiffList{{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"}}
	m := InitialModel(diffs, output.FormatOptions{Format: "sql"})
	final, err := tea.NewProgram(m, tea.WithInput(strings.NewReader("\r")), tea.WithOutput(io.Discard)).Run()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := final.(Model); !ok || !got.IsConfirmed || len(got.GetSelectedDiffs()) != 1 {
		t.Errorf("expected a confirmed model with its diff selected, got %#v", final)
	}
	if m.IsConfirmed {
		t.Error("the initial model changed")
	}
}
