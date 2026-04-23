package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saadnvd1/xpass/internal/crypto"
)

func (m Model) updateGenerate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.view = viewList
			return m, nil

		case "enter", "g":
			pw, err := crypto.GeneratePassword(m.genLength, m.genUpper, m.genLower, m.genNumbers, m.genSymbols)
			if err != nil {
				m.statusMsg = "Error: " + err.Error()
				return m, nil
			}
			m.genResult = pw
			return m, nil

		case "c":
			if m.genResult != "" {
				return m, m.copyToClipboard(m.genResult, "generated password")
			}
			return m, nil

		case "p":
			// Generate passphrase
			pp, err := crypto.GeneratePassphrase(4, "-")
			if err != nil {
				m.statusMsg = "Error: " + err.Error()
				return m, nil
			}
			m.genResult = pp
			return m, nil

		case "+", "=":
			if m.genLength < 64 {
				m.genLength++
			}
			return m, nil

		case "-":
			if m.genLength > 8 {
				m.genLength--
			}
			return m, nil

		case "u":
			m.genUpper = !m.genUpper
			return m, nil

		case "l":
			m.genLower = !m.genLower
			return m, nil

		case "n":
			m.genNumbers = !m.genNumbers
			return m, nil

		case "s":
			m.genSymbols = !m.genSymbols
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewGenerate() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Password Generator") + "\n\n")

	// Options
	b.WriteString(fmt.Sprintf("  Length: %s %d %s\n",
		helpKeyStyle.Render("-"), m.genLength, helpKeyStyle.Render("+")))

	b.WriteString(fmt.Sprintf("  %s Uppercase  %s\n",
		helpKeyStyle.Render("u"), toggleStr(m.genUpper)))
	b.WriteString(fmt.Sprintf("  %s Lowercase  %s\n",
		helpKeyStyle.Render("l"), toggleStr(m.genLower)))
	b.WriteString(fmt.Sprintf("  %s Numbers    %s\n",
		helpKeyStyle.Render("n"), toggleStr(m.genNumbers)))
	b.WriteString(fmt.Sprintf("  %s Symbols    %s\n",
		helpKeyStyle.Render("s"), toggleStr(m.genSymbols)))

	b.WriteString("\n")

	if m.genResult != "" {
		b.WriteString(boxStyle.Render(m.genResult) + "\n")
	}

	if m.statusMsg != "" {
		b.WriteString("\n" + successStyle.Render("  "+m.statusMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(helpBar(
		"enter", "generate",
		"p", "passphrase",
		"c", "copy",
		"esc", "back",
	))

	return b.String()
}

func toggleStr(on bool) string {
	if on {
		return successStyle.Render("[ON]")
	}
	return mutedStyle.Render("[off]")
}
