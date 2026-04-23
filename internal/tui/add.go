package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saadnvd1/xpass/internal/vault"
)

func (m Model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.view = viewList
			m.editingEntry = nil
			return m, nil

		case "tab", "down":
			m.addInputs[m.addFocused].Blur()
			m.addFocused = (m.addFocused + 1) % len(m.addInputs)
			m.addInputs[m.addFocused].Focus()
			return m, nil

		case "shift+tab", "up":
			m.addInputs[m.addFocused].Blur()
			m.addFocused = (m.addFocused - 1 + len(m.addInputs)) % len(m.addInputs)
			m.addInputs[m.addFocused].Focus()
			return m, nil

		case "enter":
			// Check if on last field — save
			if m.addFocused == len(m.addInputs)-1 {
				return m.saveEntry()
			}
			// Otherwise move to next
			m.addInputs[m.addFocused].Blur()
			m.addFocused = (m.addFocused + 1) % len(m.addInputs)
			m.addInputs[m.addFocused].Focus()
			return m, nil

		case "ctrl+s":
			return m.saveEntry()
		}
	}

	var cmd tea.Cmd
	m.addInputs[m.addFocused], cmd = m.addInputs[m.addFocused].Update(msg)
	return m, cmd
}

func (m Model) saveEntry() (tea.Model, tea.Cmd) {
	name := m.addInputs[0].Value()
	if name == "" {
		m.statusMsg = "Name is required"
		return m, clearStatusAfter(3000000000) // 3s
	}

	// Parse tags from last input
	tagsStr := m.addInputs[len(m.addInputs)-1].Value()
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	entry := vault.Entry{
		Name: name,
		Type: m.addType,
		Tags: tags,
	}

	switch m.addType {
	case vault.TypeLogin:
		if len(m.addInputs) > 1 {
			entry.Username = m.addInputs[1].Value()
		}
		if len(m.addInputs) > 2 {
			entry.Email = m.addInputs[2].Value()
		}
		if len(m.addInputs) > 3 {
			entry.Password = m.addInputs[3].Value()
		}
		if len(m.addInputs) > 4 {
			entry.URL = m.addInputs[4].Value()
		}
		if len(m.addInputs) > 5 && m.addInputs[5].Value() != "" {
			entry.TOTP = &vault.TOTP{
				Secret:    m.addInputs[5].Value(),
				Algorithm: "SHA1",
				Digits:    6,
				Period:    30,
			}
		}

	case vault.TypeSecureNote:
		if len(m.addInputs) > 1 {
			entry.Content = m.addInputs[1].Value()
		}

	case vault.TypeAPIKey:
		if len(m.addInputs) > 1 {
			entry.APIKey = m.addInputs[1].Value()
		}
		if len(m.addInputs) > 2 {
			entry.APISecret = m.addInputs[2].Value()
		}
		if len(m.addInputs) > 3 {
			entry.Endpoint = m.addInputs[3].Value()
		}

	case vault.TypeSSHKey:
		if len(m.addInputs) > 1 {
			entry.PrivateKey = m.addInputs[1].Value()
		}
		if len(m.addInputs) > 2 {
			entry.Passphrase = m.addInputs[2].Value()
		}
	}

	if m.view == viewEdit && m.editingEntry != nil {
		entry.Favorite = m.editingEntry.Favorite
		_, err := m.vault.Update(m.editingEntry.ID, entry)
		if err != nil {
			m.statusMsg = "Error: " + err.Error()
			return m, clearStatusAfter(3000000000)
		}
		m.statusMsg = "Updated " + name
	} else {
		_, err := m.vault.Add(entry)
		if err != nil {
			m.statusMsg = "Error: " + err.Error()
			return m, clearStatusAfter(3000000000)
		}
		m.statusMsg = "Added " + name
	}

	m.view = viewList
	m.editingEntry = nil
	m.refreshEntries()
	return m, clearStatusAfter(3000000000)
}

func (m Model) viewAdd() string {
	var b strings.Builder

	action := "Add"
	if m.view == viewEdit {
		action = "Edit"
	}

	b.WriteString(titleStyle.Render(action+" "+m.addType.DisplayName()) + "\n\n")

	for i, input := range m.addInputs {
		prefix := "  "
		if i == m.addFocused {
			prefix = promptStyle.Render("> ")
		}
		b.WriteString(prefix + input.View() + "\n")
	}

	if m.statusMsg != "" {
		b.WriteString("\n" + dangerStyle.Render("  "+m.statusMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(helpBar(
		"tab", "next field",
		"shift+tab", "prev",
		"ctrl+s", "save",
		"esc", "cancel",
	))

	return b.String()
}
