package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // violet
	colorSecondary = lipgloss.Color("#6366F1") // indigo
	colorSuccess   = lipgloss.Color("#10B981") // green
	colorDanger    = lipgloss.Color("#EF4444") // red
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorText      = lipgloss.Color("#E5E7EB") // light gray
	colorBg        = lipgloss.Color("#111827") // dark bg
	colorBgAlt     = lipgloss.Color("#1F2937") // slightly lighter

	// Base styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(colorText)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	dangerStyle = lipgloss.NewStyle().
			Foreground(colorDanger)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	// Input styles
	promptStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// Entry detail styles
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(16)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText)

	secretStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	// Status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	// Help
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Box
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	// Tag
	tagStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Background(lipgloss.Color("#1E1B4B")).
			Padding(0, 1)

	// Type badge
	typeBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(colorPrimary).
			Padding(0, 1).
			Bold(true)
)
