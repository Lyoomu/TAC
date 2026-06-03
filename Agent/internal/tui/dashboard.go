package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dashboardModel struct {
	ctx    *AppContext
	width  int
	height int

	modelCount     int
	roleCount      int
	toolCount      int
	componentCount int
}

func newDashboardModel(ctx *AppContext) *dashboardModel {
	return &dashboardModel{ctx: ctx}
}

func (d *dashboardModel) Init() tea.Cmd {
	return d.refresh()
}

func (d *dashboardModel) setSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *dashboardModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return dashboardDataMsg{}
	}
}

type dashboardDataMsg struct{}

func (d *dashboardModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case dashboardDataMsg:

		if d.ctx.ModelsEngine != nil {
			if list, err := d.ctx.ModelsEngine.List(); err == nil {
				d.modelCount = len(list)
			}
		}
		if d.ctx.RoleEngine != nil {
			if list, err := d.ctx.RoleEngine.List(); err == nil {
				d.roleCount = len(list)
			}
		}
		if d.ctx.ToolEngine != nil {
			d.toolCount = len(d.ctx.ToolEngine.List())
		}
		if d.ctx.ComponentEngine != nil {
			if list, err := d.ctx.ComponentEngine.List(); err == nil {
				d.componentCount = len(list)
			}
		}
	}
	return nil
}

func (d *dashboardModel) View() string {
	var b strings.Builder

	header := titleStyle.Render("  TAC Agent Dashboard")
	b.WriteString(header)
	b.WriteString("\n\n")

	cardWidth := 28
	if d.width > 0 {
		cardWidth = (d.width - 12) / 4
		if cardWidth < 20 {
			cardWidth = 20
		}
	}

	cardStyle := panelStyle.Width(cardWidth)

	modelsCard := cardStyle.Render(
		labelStyle.Render("Models") + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d registered", d.modelCount)),
	)

	rolesCard := cardStyle.Render(
		labelStyle.Render("Roles") + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d defined", d.roleCount)),
	)

	toolsCard := cardStyle.Render(
		labelStyle.Render("Tools") + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d available", d.toolCount)),
	)

	componentsCard := cardStyle.Render(
		labelStyle.Render("Components") + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d created", d.componentCount)),
	)

	row := lipgloss.JoinHorizontal(lipgloss.Top, modelsCard, " ", rolesCard, " ", toolsCard, " ", componentsCard)
	b.WriteString(row)
	b.WriteString("\n\n")

	b.WriteString(subtitleStyle.Render("Quick Actions"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  [Tab] Switch tabs  [/] Enter command  [Enter] on Role to start chat"))
	b.WriteString("\n\n")

	b.WriteString(subtitleStyle.Render("Server"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  Use 'server start' to launch gRPC server"))

	return b.String()
}
