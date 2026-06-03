package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpModel struct {
	width  int
	height int
	show   bool
}

func newHelpModel() *helpModel {
	return &helpModel{}
}

func (h *helpModel) setSize(w, hgt int) {
	h.width = w
	h.height = hgt
}

func (h *helpModel) Toggle() {
	h.show = !h.show
}

func (h *helpModel) Hide() {
	h.show = false
}

func (h *helpModel) IsVisible() bool {
	return h.show
}

func (h *helpModel) Update(msg tea.Msg) {

}

func (h *helpModel) View() string {
	if !h.show {
		return ""
	}

	content := helpContent
	contentWidth := minInt(60, h.width-4)
	if contentWidth < 30 {
		contentWidth = 30
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(contentWidth)

	rendered := style.Render(content)

	// Use lipgloss.Place for proper centering
	return lipgloss.Place(
		h.width, h.height,
		lipgloss.Center, lipgloss.Center,
		rendered,
	)
}

var helpContent = fmt.Sprintf(`%s

%s

Navigation:
  Tab / Shift+Tab    Switch tabs
  ?                  Toggle this help
  Q / Esc            Quit or go back

Commands:
  /  or :            Focus command input
  Enter              Execute command
  Esc                Cancel input

%s
  Enter              Start chat (Servers tab)
  Esc                Return to dashboard
  PgUp / PgDn        Scroll chat
  Ctrl+C             Interrupt generation
  Ctrl+F             Toggle reasoning fold

%s
  c                  Connect to server
  l                  Load role from server
  u                  Update roles from server
  d                  Remove server

%s
  c                  Create trigger
  e                  Edit trigger
  d                  Delete trigger
  s                  Start trigger
  x                  Stop trigger
  r                  Run (direct) trigger

%s
  c                  Create event
  e                  Edit event
  d                  Delete event
  Enter              View details

%s
	c                  Create env preset
	e                  Edit env preset
	d                  Delete env preset
	Enter              View details

%s
  s                  Start daemon
  x                  Stop daemon
  r                  Refresh status
`,
	titleStyle.Render("  TAC Trigger TUI Help"),
	subtitleStyle.Render("General"),
	subtitleStyle.Render("Chat"),
	subtitleStyle.Render("Servers Tab"),
	subtitleStyle.Render("Triggers Tab"),
	subtitleStyle.Render("Events Tab"),
	subtitleStyle.Render("Env Presets Tab"),
	subtitleStyle.Render("Daemon Tab"),
)
