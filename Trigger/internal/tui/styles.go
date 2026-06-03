package tui

import "github.com/charmbracelet/lipgloss"

var (
	primaryColor   = lipgloss.Color("#2563EB") // blue
	secondaryColor = lipgloss.Color("#06B6D4") // cyan
	accentColor    = lipgloss.Color("#F59E0B") // amber
	successColor   = lipgloss.Color("#10B981") // green
	errorColor     = lipgloss.Color("#EF4444") // red
	mutedColor     = lipgloss.Color("#6B7280") // gray

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor)

	statusBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(mutedColor).
			Padding(0, 1)

	statusMsgStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	hintStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(1, 2)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(secondaryColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(mutedColor)

	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	tableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#374151"))

	chatUserStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	chatAssistantStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	chatInputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(mutedColor).
			Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)
