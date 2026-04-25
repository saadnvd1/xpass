package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/saadnvd1/xpass/internal/clipboard"
	"github.com/saadnvd1/xpass/internal/vault"
)

// View represents the current TUI screen
type view int

const (
	viewUnlock view = iota
	viewList
	viewDetail
	viewAdd
	viewEdit
	viewGenerate
	viewConfirmDelete
	viewRecoveryImport
)

// Model is the top-level Bubble Tea model
type Model struct {
	vault  *vault.Vault
	view   view
	width  int
	height int

	// Unlock
	passwordInput textinput.Model
	unlockError   string
	needsInit     bool

	// List
	entries     []vault.Entry
	cursor      int
	searchInput textinput.Model
	searching   bool
	searchQuery string

	// Detail
	selected       *vault.Entry
	showSecret     bool
	detailScroll   int
	detailMaxScroll int

	// Add/Edit
	addInputs    []textinput.Model
	addFocused   int
	addType      vault.EntryType
	editingEntry *vault.Entry

	// Generate password
	genLength  int
	genUpper   bool
	genLower   bool
	genNumbers bool
	genSymbols bool
	genResult  string

	// Status message (temporary)
	statusMsg     string
	statusTimeout time.Time

	// Confirm delete
	deleteTarget *vault.Entry

	// Recovery code import
	recoveryInput textinput.Model
}

// NewModel creates the initial TUI model
func NewModel(v *vault.Vault) Model {
	pi := textinput.New()
	pi.Placeholder = "Master password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '*'
	pi.Focus()
	pi.CharLimit = 128
	pi.Width = 40

	si := textinput.New()
	si.Placeholder = "Search entries..."
	si.CharLimit = 64
	si.Width = 40

	m := Model{
		vault:         v,
		view:          viewUnlock,
		passwordInput: pi,
		searchInput:   si,
		needsInit:     !v.Exists(),
		genLength:     20,
		genUpper:      true,
		genLower:      true,
		genNumbers:    true,
		genSymbols:    true,
	}

	return m
}

type statusClearMsg struct{}
type copiedMsg struct{ field string }

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case statusClearMsg:
		m.statusMsg = ""
		return m, nil

	case copiedMsg:
		m.statusMsg = "Copied " + msg.field + " to clipboard (clears in 30s)"
		return m, clearStatusAfter(3 * time.Second)

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.view {
	case viewUnlock:
		return m.updateUnlock(msg)
	case viewList:
		return m.updateList(msg)
	case viewDetail:
		return m.updateDetail(msg)
	case viewAdd, viewEdit:
		return m.updateAdd(msg)
	case viewGenerate:
		return m.updateGenerate(msg)
	case viewConfirmDelete:
		return m.updateConfirmDelete(msg)
	case viewRecoveryImport:
		return m.updateRecoveryImport(msg)
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	switch m.view {
	case viewUnlock:
		return m.viewUnlock()
	case viewList:
		return m.viewList()
	case viewDetail:
		return m.viewDetail()
	case viewAdd, viewEdit:
		return m.viewAdd()
	case viewGenerate:
		return m.viewGenerate()
	case viewConfirmDelete:
		return m.viewConfirmDelete()
	case viewRecoveryImport:
		return m.viewRecoveryImport()
	}
	return ""
}

// setStatus shows a temporary status message
func (m *Model) setStatus(msg string) tea.Cmd {
	m.statusMsg = msg
	return clearStatusAfter(3 * time.Second)
}

// copyToClipboard copies text and shows status
func (m *Model) copyToClipboard(text, field string) tea.Cmd {
	clipboard.CopyWithClear(text, 30*time.Second)
	return func() tea.Msg {
		return copiedMsg{field: field}
	}
}

// refreshEntries reloads the entry list based on current search
func (m *Model) refreshEntries() {
	if m.searchQuery != "" {
		m.entries = m.vault.Search(m.searchQuery)
	} else {
		m.entries = m.vault.Entries()
	}
	if m.cursor >= len(m.entries) {
		m.cursor = max(0, len(m.entries)-1)
	}
}
