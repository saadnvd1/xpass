package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/saadnvd1/xpass/internal/vault"
)

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateListSearch(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.refreshEntries()
				return m, nil
			}
			m.vault.Lock()
			m.view = viewUnlock
			m.passwordInput.Focus()
			return m, nil

		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "g":
			m.cursor = 0
			return m, nil

		case "G":
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
			return m, nil

		case "enter", "l", "right":
			if len(m.entries) > 0 {
				entry := m.entries[m.cursor]
				m.selected = &entry
				m.showSecret = false
				m.detailScroll = 0
				m.view = viewDetail
				m.vault.TrackAccess(entry.ID)
				// Start TOTP ticker if entry has TOTP
				if entry.TOTP != nil {
					return m, totpTick()
				}
			}
			return m, nil

		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, nil

		case "a":
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeLogin) // default to login
			return m, nil

		case "n":
			// Quick add secure note
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeSecureNote)
			return m, nil

		case "p":
			// Generate password
			m.view = viewGenerate
			m.genResult = ""
			return m, nil

		case "c":
			// Quick copy password of selected entry
			if len(m.entries) > 0 {
				entry := m.entries[m.cursor]
				pw := entry.Password
				if pw == "" {
					pw = entry.APIKey
				}
				if pw != "" {
					m.vault.TrackAccess(entry.ID)
					return m, m.copyToClipboard(pw, "password")
				}
			}
			return m, nil

		case "f":
			// Toggle favorite
			if len(m.entries) > 0 {
				entry := m.entries[m.cursor]
				entry.Favorite = !entry.Favorite
				m.vault.Update(entry.ID, entry)
				m.refreshEntries()
			}
			return m, nil

		case "d":
			// Delete entry
			if len(m.entries) > 0 {
				entry := m.entries[m.cursor]
				m.deleteTarget = &entry
				m.view = viewConfirmDelete
			}
			return m, nil

		case "1":
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeLogin)
			return m, nil
		case "2":
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeAPIKey)
			return m, nil
		case "3":
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeSSHKey)
			return m, nil
		case "4":
			m.view = viewAdd
			m.editingEntry = nil
			m.initAddInputs(vault.TypeSecureNote)
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateListSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.searching = false
			m.searchQuery = m.searchInput.Value()
			m.searchInput.Blur()
			return m, nil

		case "esc":
			m.searching = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			m.refreshEntries()
			m.cursor = 0
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	// Live filter as you type
	m.searchQuery = m.searchInput.Value()
	m.refreshEntries()
	m.cursor = 0
	return m, cmd
}

func (m Model) viewList() string {
	var b strings.Builder

	// Header
	count := len(m.entries)
	total := m.vault.Count()
	header := titleStyle.Render("xpass")
	if m.searchQuery != "" {
		header += mutedStyle.Render(fmt.Sprintf("  %d/%d matching \"%s\"", count, total, m.searchQuery))
	} else {
		header += mutedStyle.Render(fmt.Sprintf("  %d entries", total))
	}
	b.WriteString(header + "\n")

	// Search bar
	if m.searching {
		b.WriteString("  " + promptStyle.Render("/") + " " + m.searchInput.View() + "\n\n")
	} else if m.searchQuery != "" {
		b.WriteString("  " + mutedStyle.Render("filter: "+m.searchQuery) + "\n\n")
	} else {
		b.WriteString("\n")
	}

	// Entry list
	if len(m.entries) == 0 {
		if m.searchQuery != "" {
			b.WriteString(mutedStyle.Render("  No entries match your search.\n"))
		} else {
			b.WriteString(mutedStyle.Render("  No entries yet. Press 'a' to add one.\n"))
		}
	} else {
		visible := m.height - 8 // rough estimate of available lines
		if visible < 5 {
			visible = 5
		}

		start := 0
		if m.cursor >= visible {
			start = m.cursor - visible + 1
		}
		end := start + visible
		if end > len(m.entries) {
			end = len(m.entries)
		}

		for i := start; i < end; i++ {
			entry := m.entries[i]
			cursor := "  "
			nameStyle := normalStyle
			subStyle := mutedStyle

			if i == m.cursor {
				cursor = selectedStyle.Render("> ")
				nameStyle = selectedStyle
				subStyle = lipgloss.NewStyle().Foreground(colorSecondary)
			}

			// Type indicator
			typeIcon := typeIcon(entry.Type)

			// Favorite star
			star := ""
			if entry.Favorite {
				star = warningStyle.Render(" *")
			}

			name := nameStyle.Render(entry.Name)
			subtitle := subStyle.Render(entry.Subtitle())

			line := fmt.Sprintf("%s%s %s%s  %s", cursor, typeIcon, name, star, subtitle)
			b.WriteString(line + "\n")
		}
	}

	// Status
	if m.statusMsg != "" {
		b.WriteString("\n" + successStyle.Render("  "+m.statusMsg))
	}

	// Help bar
	b.WriteString("\n\n")
	b.WriteString(helpBar(
		"j/k", "navigate",
		"enter", "view",
		"a", "add",
		"/", "search",
		"c", "copy pw",
		"p", "generate",
		"d", "delete",
		"q", "lock",
	))

	return b.String()
}

func typeIcon(t vault.EntryType) string {
	switch t {
	case vault.TypeLogin:
		return mutedStyle.Render("[L]")
	case vault.TypeCreditCard:
		return mutedStyle.Render("[C]")
	case vault.TypeSecureNote:
		return mutedStyle.Render("[N]")
	case vault.TypeSSHKey:
		return mutedStyle.Render("[S]")
	case vault.TypeAPIKey:
		return mutedStyle.Render("[A]")
	case vault.TypeDatabase:
		return mutedStyle.Render("[D]")
	case vault.TypeServer:
		return mutedStyle.Render("[V]")
	case vault.TypeCryptoWallet:
		return mutedStyle.Render("[W]")
	default:
		return mutedStyle.Render("[?]")
	}
}

func helpBar(pairs ...string) string {
	var parts []string
	for i := 0; i < len(pairs)-1; i += 2 {
		parts = append(parts, helpKeyStyle.Render(pairs[i])+" "+helpDescStyle.Render(pairs[i+1]))
	}
	return "  " + strings.Join(parts, "  ")
}
