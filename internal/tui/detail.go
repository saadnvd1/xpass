package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saadnvd1/xpass/internal/otp"
	"github.com/saadnvd1/xpass/internal/vault"
)

type totpTickMsg struct{}

func totpTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return totpTickMsg{}
	})
}

func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case totpTickMsg:
		// Keep ticking while viewing an entry with TOTP
		if m.view == viewDetail && m.selected != nil && m.selected.TOTP != nil {
			return m, totpTick()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "h", "left":
			m.view = viewList
			m.selected = nil
			return m, nil

		case "j", "down":
			m.detailScroll++
			return m, nil

		case "k", "up":
			if m.detailScroll > 0 {
				m.detailScroll--
			}
			return m, nil

		case "g":
			m.detailScroll = 0
			return m, nil

		case "s":
			m.showSecret = !m.showSecret
			return m, nil

		case "c":
			if m.selected != nil {
				pw := m.selected.Password
				if pw == "" {
					pw = m.selected.APIKey
				}
				if pw == "" {
					pw = m.selected.PrivateKey
				}
				if pw != "" {
					return m, m.copyToClipboard(pw, "secret")
				}
			}
			return m, nil

		case "t":
			// Copy current TOTP code
			if m.selected != nil && m.selected.TOTP != nil {
				code, _, _ := otp.Generate(
					m.selected.TOTP.Secret,
					m.selected.TOTP.Algorithm,
					m.selected.TOTP.Digits,
					m.selected.TOTP.Period,
				)
				return m, m.copyToClipboard(code, "TOTP code")
			}
			return m, nil

		case "u":
			if m.selected != nil && m.selected.Username != "" {
				return m, m.copyToClipboard(m.selected.Username, "username")
			}
			return m, nil

		case "e":
			if m.selected != nil {
				m.view = viewEdit
				m.editingEntry = m.selected
				m.initEditInputs()
			}
			return m, nil

		case "d":
			if m.selected != nil {
				m.deleteTarget = m.selected
				m.view = viewConfirmDelete
			}
			return m, nil

		case "f":
			if m.selected != nil {
				m.selected.Favorite = !m.selected.Favorite
				m.vault.Update(m.selected.ID, *m.selected)
				m.refreshEntries()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewDetail() string {
	if m.selected == nil {
		return "No entry selected"
	}

	e := m.selected
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render(e.Name) + "\n")
	b.WriteString(typeBadgeStyle.Render(e.Type.DisplayName()))
	if e.Favorite {
		b.WriteString(warningStyle.Render(" *"))
	}
	b.WriteString("\n\n")

	// Fields based on type
	switch e.Type {
	case vault.TypeLogin:
		if e.Username != "" {
			b.WriteString(field("Username", e.Username, false))
		}
		if e.Email != "" {
			b.WriteString(field("Email", e.Email, false))
		}
		b.WriteString(field("Password", e.Password, !m.showSecret))
		if e.URL != "" {
			b.WriteString(field("URL", e.URL, false))
		}
		if e.TOTP != nil {
			code, remaining, period := otp.Generate(
				e.TOTP.Secret, e.TOTP.Algorithm, e.TOTP.Digits, e.TOTP.Period,
			)
			timeBar := otp.TimeBar(remaining, period)
			totpDisplay := code + "  " + mutedStyle.Render(timeBar)
			b.WriteString(labelStyle.Render("TOTP") + totpDisplay + "\n")
			if m.showSecret {
				b.WriteString(labelStyle.Render("TOTP Secret") + secretStyle.Render(e.TOTP.Secret) + "\n")
			}
		}

	case vault.TypeSecureNote:
		b.WriteString(field("Content", e.Content, !m.showSecret))

	case vault.TypeAPIKey:
		b.WriteString(field("API Key", e.APIKey, !m.showSecret))
		if e.APISecret != "" {
			b.WriteString(field("API Secret", e.APISecret, !m.showSecret))
		}
		if e.Endpoint != "" {
			b.WriteString(field("Endpoint", e.Endpoint, false))
		}

	case vault.TypeSSHKey:
		b.WriteString(field("Key Type", e.KeyType, false))
		b.WriteString(field("Private Key", e.PrivateKey, !m.showSecret))
		if e.PublicKey != "" {
			b.WriteString(field("Public Key", e.PublicKey, false))
		}
		if e.Passphrase != "" {
			b.WriteString(field("Passphrase", e.Passphrase, !m.showSecret))
		}

	case vault.TypeCreditCard:
		b.WriteString(field("Cardholder", e.CardholderName, false))
		b.WriteString(field("Number", e.CardNumber, !m.showSecret))
		b.WriteString(field("Expiry", e.ExpiryMonth+"/"+e.ExpiryYear, false))
		b.WriteString(field("CVV", e.CVV, !m.showSecret))
		if e.PIN != "" {
			b.WriteString(field("PIN", e.PIN, !m.showSecret))
		}

	case vault.TypeDatabase:
		b.WriteString(field("Type", e.DBType, false))
		b.WriteString(field("Host", e.Host, false))
		if e.Port > 0 {
			b.WriteString(field("Port", fmt.Sprintf("%d", e.Port), false))
		}
		b.WriteString(field("Database", e.Database, false))
		b.WriteString(field("Username", e.Username, false))
		b.WriteString(field("Password", e.Password, !m.showSecret))

	case vault.TypeServer:
		b.WriteString(field("Host", e.Host, false))
		b.WriteString(field("Protocol", e.Protocol, false))
		b.WriteString(field("Username", e.Username, false))
		if e.Password != "" {
			b.WriteString(field("Password", e.Password, !m.showSecret))
		}
	}

	// Notes
	if e.Notes != "" {
		b.WriteString("\n" + labelStyle.Render("Notes") + "\n")
		b.WriteString(mutedStyle.Render(e.Notes) + "\n")
	}

	// Tags
	if len(e.Tags) > 0 {
		b.WriteString("\n")
		for _, tag := range e.Tags {
			b.WriteString(tagStyle.Render(tag) + " ")
		}
		b.WriteString("\n")
	}

	// Metadata
	b.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("Created: %s  |  Updated: %s  |  v%d",
		e.CreatedAt[:10], e.UpdatedAt[:10], e.Version)))

	// Status
	if m.statusMsg != "" {
		b.WriteString("\n\n" + successStyle.Render(m.statusMsg))
	}

	// Help
	b.WriteString("\n\n")
	helpItems := []string{
		"j/k", "scroll",
		"s", "show/hide",
		"c", "copy secret",
		"u", "copy user",
	}
	if e.TOTP != nil {
		helpItems = append(helpItems, "t", "copy TOTP")
	}
	helpItems = append(helpItems, "e", "edit", "d", "delete", "f", "fav", "esc", "back")
	helpLine := helpBar(helpItems...)

	// Apply scrolling — split into lines, show visible window
	content := b.String()
	lines := strings.Split(content, "\n")

	// Reserve lines for help bar
	visible := m.height - 3
	if visible < 5 {
		visible = 5
	}

	// Clamp scroll
	maxScroll := len(lines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.detailScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}

	end := scroll + visible
	if end > len(lines) {
		end = len(lines)
	}

	visibleLines := lines[scroll:end]
	result := strings.Join(visibleLines, "\n")

	// Scroll indicator
	if maxScroll > 0 {
		scrollPct := ""
		if scroll == 0 {
			scrollPct = mutedStyle.Render("  [top]")
		} else if scroll >= maxScroll {
			scrollPct = mutedStyle.Render("  [bottom]")
		} else {
			pct := scroll * 100 / maxScroll
			scrollPct = mutedStyle.Render(fmt.Sprintf("  [%d%%]", pct))
		}
		result += scrollPct
	}

	result += "\n" + helpLine

	return result
}

func field(label, value string, masked bool) string {
	l := labelStyle.Render(label)
	if masked {
		return l + secretStyle.Render(strings.Repeat("*", min(len(value), 20))) + "\n"
	}

	// Wrap long values across multiple lines
	maxWidth := 60
	if len(value) <= maxWidth {
		return l + valueStyle.Render(value) + "\n"
	}

	// First line with label
	var result strings.Builder
	result.WriteString(l)

	indent := strings.Repeat(" ", 16) // match labelStyle width
	remaining := value
	first := true

	for len(remaining) > 0 {
		lineWidth := maxWidth
		if first {
			first = false
		} else {
			result.WriteString(indent)
		}

		if len(remaining) <= lineWidth {
			result.WriteString(valueStyle.Render(remaining) + "\n")
			break
		}

		// Try to break at a space
		cut := lineWidth
		for cut > lineWidth/2 {
			if remaining[cut] == ' ' {
				break
			}
			cut--
		}
		if cut <= lineWidth/2 {
			cut = lineWidth // no good break point, hard cut
		}

		result.WriteString(valueStyle.Render(remaining[:cut]) + "\n")
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}

	return result.String()
}

func (m *Model) initEditInputs() {
	if m.editingEntry == nil {
		return
	}

	m.initAddInputs(m.editingEntry.Type)

	// Pre-fill inputs with existing values
	e := m.editingEntry
	m.addInputs[0].SetValue(e.Name) // name is always first

	switch e.Type {
	case vault.TypeLogin:
		if len(m.addInputs) > 1 {
			m.addInputs[1].SetValue(e.Username)
		}
		if len(m.addInputs) > 2 {
			m.addInputs[2].SetValue(e.Email)
		}
		if len(m.addInputs) > 3 {
			m.addInputs[3].SetValue(e.Password)
		}
		if len(m.addInputs) > 4 {
			m.addInputs[4].SetValue(e.URL)
		}
		if len(m.addInputs) > 5 && e.TOTP != nil {
			m.addInputs[5].SetValue(e.TOTP.Secret)
		}

	case vault.TypeSecureNote:
		if len(m.addInputs) > 1 {
			m.addInputs[1].SetValue(e.Content)
		}

	case vault.TypeAPIKey:
		if len(m.addInputs) > 1 {
			m.addInputs[1].SetValue(e.APIKey)
		}
		if len(m.addInputs) > 2 {
			m.addInputs[2].SetValue(e.APISecret)
		}
		if len(m.addInputs) > 3 {
			m.addInputs[3].SetValue(e.Endpoint)
		}

	case vault.TypeSSHKey:
		if len(m.addInputs) > 1 {
			m.addInputs[1].SetValue(e.PrivateKey)
		}
		if len(m.addInputs) > 2 {
			m.addInputs[2].SetValue(e.Passphrase)
		}
	}

	// Tags (always last)
	if len(m.addInputs) > 0 && len(e.Tags) > 0 {
		m.addInputs[len(m.addInputs)-1].SetValue(strings.Join(e.Tags, ", "))
	}
}
