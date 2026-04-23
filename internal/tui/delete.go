package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			if m.deleteTarget != nil {
				name := m.deleteTarget.Name
				m.vault.Delete(m.deleteTarget.ID)
				m.deleteTarget = nil
				m.selected = nil
				m.view = viewList
				m.refreshEntries()
				m.statusMsg = fmt.Sprintf("Deleted %s", name)
				return m, clearStatusAfter(3000000000)
			}
			return m, nil

		case "n", "N", "esc":
			target := m.deleteTarget
			m.deleteTarget = nil
			// Go back to where we came from
			if m.selected != nil && m.selected.ID == target.ID {
				m.view = viewDetail
			} else {
				m.view = viewList
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewConfirmDelete() string {
	if m.deleteTarget == nil {
		return ""
	}

	var b strings.Builder

	b.WriteString(dangerStyle.Render("Delete entry?") + "\n\n")
	b.WriteString(fmt.Sprintf("  Name: %s\n", m.deleteTarget.Name))
	b.WriteString(fmt.Sprintf("  Type: %s\n", m.deleteTarget.Type.DisplayName()))
	b.WriteString("\n")
	b.WriteString("  This cannot be undone.\n\n")
	b.WriteString(helpBar("y", "delete", "n", "cancel"))

	return b.String()
}
