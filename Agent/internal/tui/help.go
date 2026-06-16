package tui

import (
	"fmt"
	"strings"

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

	var b strings.Builder

	content := helpContent
	contentWidth := min(60, h.width-4)
	contentHeight := min(len(strings.Split(content, "\n"))+4, h.height-4)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Background(lipgloss.Color("#111827")).
		Padding(1, 2).
		Width(contentWidth)

	rendered := style.Render(content)

	vertPad := (h.height - contentHeight) / 2
	if vertPad < 0 {
		vertPad = 0
	}
	for i := 0; i < vertPad; i++ {
		b.WriteString("\n")
	}

	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		pad := (h.width - lipgloss.Width(line)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
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
  Enter              Start chat (Roles tab)
  Esc                Return to dashboard
  PgUp / PgDn        Scroll chat
  Ctrl+C             Interrupt generation

%s
  c                  Create model
  e                  Edit model
  d                  Delete model
  Enter              View details

%s
  c                  Create role
  e                  Edit role
  d                  Delete role
  t                  Start chat
  Enter              View details

%s
  c                  Create component
  e                  Edit component
  d                  Delete component
  Enter              View details

%s
  Enter              Toggle detail view

%s
  s                  Start server
  x                  Stop server
`,
	titleStyle.Render("  TAC Agent TUI Help"),
	subtitleStyle.Render("General"),
	subtitleStyle.Render("Chat"),
	subtitleStyle.Render("Models Tab"),
	subtitleStyle.Render("Roles Tab"),
	subtitleStyle.Render("Components Tab"),
	subtitleStyle.Render("Tools Tab"),
	subtitleStyle.Render("Server Tab"),
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
