package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/meru143/dbdiff/pkg/types"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00E676")).
				Bold(true)

	unselectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA"))

	checkedBox   = "[x]"
	uncheckedBox = "[ ]"

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)

	previewTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00BFFF")).
				Bold(true).
				MarginBottom(1)

	basePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1)

	containerStyle = lipgloss.NewStyle().
			Align(lipgloss.Top)
)

type Model struct {
	diffs       types.DiffList
	cursor      int
	selected    map[int]struct{} // Set of selected diff indices
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	formatter   *output.Formatter
	IsConfirmed bool
}

// InitialModel configures the starting TUI state
func InitialModel(diffs types.DiffList) Model {
	// Pre-select all diffs by default
	selected := make(map[int]struct{})
	for i := range diffs {
		selected[i] = struct{}{}
	}

	return Model{
		diffs:     diffs,
		selected:  selected,
		formatter: output.NewFormatter("sql"),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewport()
			}

		case "down", "j":
			if m.cursor < len(m.diffs)-1 {
				m.cursor++
				m.updateViewport()
			}

		case " ", "space":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
			m.updateViewport()

		case "enter":
			m.IsConfirmed = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve vertical space for headers/footers
		paneHeight := msg.Height - 6

		// Split width evenly between list and preview panes
		paneWidth := (msg.Width / 2) - 4

		if !m.ready {
			m.viewport = viewport.New(paneWidth, paneHeight)
			m.ready = true
		} else {
			m.viewport.Width = paneWidth
			m.viewport.Height = paneHeight
		}

		m.updateViewport()
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	if !m.ready || len(m.diffs) == 0 {
		return
	}

	// Format left pane (List of differences)
	var listItems []string
	for i, diffItem := range m.diffs {
		cursorPtr := "  " // No cursor
		if m.cursor == i {
			cursorPtr = "> " // Render cursor indicator
		}

		checked := uncheckedBox
		if _, ok := m.selected[i]; ok {
			checked = checkedBox
		}

		label := fmt.Sprintf("%s %s %s", string(diffItem.Type), diffItem.Object, diffItem.Name)

		itemLine := fmt.Sprintf("%s%s %s", cursorPtr, checked, label)

		if m.cursor == i {
			itemLine = selectedItemStyle.Render(itemLine)
		} else {
			itemLine = unselectedItemStyle.Render(itemLine)
		}

		listItems = append(listItems, itemLine)
	}

	// Format Right Pane (SQL Preview)
	previewText := "No item selected."
	if m.cursor >= 0 && m.cursor < len(m.diffs) {
		singleDiff := types.DiffList{m.diffs[m.cursor]}
		// Capture raw SQL generation purely for display
		sqlStr, err := m.formatter.Format(singleDiff)
		if err == nil {
			previewText = string(sqlStr)
		} else {
			previewText = fmt.Sprintf("-- Error generating SQL Preview: %v", err)
		}
	}

	m.viewport.SetContent(previewText)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing...\n"
	}

	header := titleStyle.Render("DBDiff Interactive Mode")

	// Left Pane logic
	var listBuilder strings.Builder
	for i, diffItem := range m.diffs {
		cursorPtr := "  "
		if m.cursor == i {
			cursorPtr = "> "
		}

		checked := uncheckedBox
		if _, ok := m.selected[i]; ok {
			checked = checkedBox
		}

		label := fmt.Sprintf("%s %s %s", string(diffItem.Type), diffItem.Object, diffItem.Name)
		itemLine := fmt.Sprintf("%s%s %s", cursorPtr, checked, label)

		if m.cursor == i {
			itemLine = selectedItemStyle.Render(itemLine)
		} else {
			itemLine = unselectedItemStyle.Render(itemLine)
		}

		listBuilder.WriteString(itemLine + "\n")
	}

	leftPaneContent := listBuilder.String()

	// Right Pane handled implicitly by Viewport holding the SQL string

	// Compute panes dimensions
	paneWidth := (m.width / 2) - 4
	paneStyle := basePaneStyle.Width(paneWidth).Height(m.viewport.Height)

	leftPane := paneStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			previewTitleStyle.Render(" Detected Differences:"),
			leftPaneContent,
		),
	)

	rightPane := paneStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			previewTitleStyle.Render(" SQL Preview:"),
			m.viewport.View(),
		),
	)

	uiLayout := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	helpText := helpStyle.Render("↑/k: up • ↓/j: down • space: select/deselect • enter: confirm • q: quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		containerStyle.Render(uiLayout),
		helpText,
	)
}

// GetSelectedDiffs exports the actively user-checked differences
func (m Model) GetSelectedDiffs() types.DiffList {
	var results types.DiffList
	for i, d := range m.diffs {
		if _, ok := m.selected[i]; ok {
			results = append(results, d)
		}
	}
	return results
}
