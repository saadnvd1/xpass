package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/saadnvd1/xpass/internal/vault"
)

func (m Model) updateUnlock(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			pw := m.passwordInput.Value()
			if pw == "" {
				return m, nil
			}

			if m.needsInit {
				if err := m.vault.Init(pw); err != nil {
					m.unlockError = err.Error()
					return m, nil
				}
				// Unlock after init
				if err := m.vault.Unlock(pw); err != nil {
					m.unlockError = err.Error()
					return m, nil
				}
			} else {
				if err := m.vault.Unlock(pw); err != nil {
					m.unlockError = "Wrong password"
					m.passwordInput.SetValue("")
					return m, nil
				}
			}

			m.view = viewList
			m.refreshEntries()
			m.passwordInput.SetValue("")
			m.unlockError = ""
			return m, nil

		case "esc":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) viewUnlock() string {
	var s string

	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Render(`
  ██╗  ██╗██████╗  █████╗ ███████╗███████╗
  ╚██╗██╔╝██╔══██╗██╔══██╗██╔════╝██╔════╝
   ╚███╔╝ ██████╔╝███████║███████╗███████╗
   ██╔██╗ ██╔═══╝ ██╔══██║╚════██║╚════██║
  ██╔╝ ██╗██║     ██║  ██║███████║███████║
  ╚═╝  ╚═╝╚═╝     ╚═╝  ╚═╝╚══════╝╚══════╝`)

	s += logo + "\n\n"

	if m.needsInit {
		s += subtitleStyle.Render("  No vault found. Enter a master password to create one.") + "\n\n"
	} else {
		s += subtitleStyle.Render("  Enter master password to unlock") + "\n\n"
	}

	s += "  " + m.passwordInput.View() + "\n\n"

	if m.unlockError != "" {
		s += "  " + dangerStyle.Render(m.unlockError) + "\n\n"
	}

	s += mutedStyle.Render("  enter") + " unlock  " +
		mutedStyle.Render("esc") + " quit"

	return s
}

func (m *Model) initAddInputs(entryType vault.EntryType) {
	_ = fmt.Sprintf // avoid unused import

	m.addType = entryType
	m.addFocused = 0

	// Create inputs based on entry type
	var inputs []textinput.Model

	name := textinput.New()
	name.Placeholder = "Name (e.g. GitHub, AWS)"
	name.CharLimit = 64
	name.Width = 40
	name.Focus()
	inputs = append(inputs, name)

	switch entryType {
	case vault.TypeLogin:
		username := textinput.New()
		username.Placeholder = "Username"
		username.CharLimit = 128
		username.Width = 40
		inputs = append(inputs, username)

		email := textinput.New()
		email.Placeholder = "Email"
		email.CharLimit = 128
		email.Width = 40
		inputs = append(inputs, email)

		pw := textinput.New()
		pw.Placeholder = "Password"
		pw.EchoMode = textinput.EchoPassword
		pw.EchoCharacter = '*'
		pw.CharLimit = 256
		pw.Width = 40
		inputs = append(inputs, pw)

		url := textinput.New()
		url.Placeholder = "URL"
		url.CharLimit = 256
		url.Width = 40
		inputs = append(inputs, url)

		totp := textinput.New()
		totp.Placeholder = "TOTP secret (optional)"
		totp.CharLimit = 128
		totp.Width = 40
		inputs = append(inputs, totp)

	case vault.TypeSecureNote:
		content := textinput.New()
		content.Placeholder = "Note content"
		content.CharLimit = 1024
		content.Width = 40
		inputs = append(inputs, content)

	case vault.TypeAPIKey:
		key := textinput.New()
		key.Placeholder = "API Key"
		key.EchoMode = textinput.EchoPassword
		key.EchoCharacter = '*'
		key.CharLimit = 256
		key.Width = 40
		inputs = append(inputs, key)

		secret := textinput.New()
		secret.Placeholder = "API Secret (optional)"
		secret.EchoMode = textinput.EchoPassword
		secret.EchoCharacter = '*'
		secret.CharLimit = 256
		secret.Width = 40
		inputs = append(inputs, secret)

		endpoint := textinput.New()
		endpoint.Placeholder = "Endpoint URL"
		endpoint.CharLimit = 256
		endpoint.Width = 40
		inputs = append(inputs, endpoint)

	case vault.TypeSSHKey:
		privKey := textinput.New()
		privKey.Placeholder = "Private key path or content"
		privKey.CharLimit = 4096
		privKey.Width = 40
		inputs = append(inputs, privKey)

		passphrase := textinput.New()
		passphrase.Placeholder = "Passphrase (optional)"
		passphrase.EchoMode = textinput.EchoPassword
		passphrase.EchoCharacter = '*'
		passphrase.CharLimit = 256
		passphrase.Width = 40
		inputs = append(inputs, passphrase)
	}

	tags := textinput.New()
	tags.Placeholder = "Tags (comma-separated)"
	tags.CharLimit = 128
	tags.Width = 40
	inputs = append(inputs, tags)

	m.addInputs = inputs
}
