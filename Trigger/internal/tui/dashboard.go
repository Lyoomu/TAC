package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
)

type dashboardModel struct {
	ctx    *AppContext
	width  int
	height int

	serverCount   int
	roleCount     int
	daemonRunning bool
	activeWS      string
	activeWSPath  string
	scanResult    string
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
		return triggerDashDataMsg{}
	}
}

type triggerDashDataMsg struct{}

func (d *dashboardModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case triggerDashDataMsg:
		if d.ctx.ServerEngine != nil {
			d.serverCount = len(d.ctx.ServerEngine.List())
			d.roleCount = len(d.ctx.ServerEngine.GetLoadedRoles())
		}
		d.daemonRunning = daemon.IsDaemonRunning()
		if d.ctx.WorkspaceEngine != nil {
			if ws, err := d.ctx.WorkspaceEngine.GetActive(); err == nil {
				d.activeWS = ws.Name
				d.activeWSPath = ws.Path
			}
		}
	}
	return nil
}

func (d *dashboardModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  TAC Trigger Dashboard"))
	b.WriteString("\n\n")

	// Responsive card layout: stack vertically if terminal is narrow
	cardWidth := 30
	if d.width > 0 {
		cardWidth = (d.width - 8) / 3
		if cardWidth < 22 {
			cardWidth = 22
		}
	}
	useHorizontal := d.width >= 76

	cardStyle := panelStyle.Width(cardWidth)

	wsInfo := "Not set"
	if d.activeWS != "" {
		wsInfo = d.activeWS
		if d.activeWSPath != "" {
			wsInfo += "\n  " + truncate(d.activeWSPath, cardWidth-6)
		}
	}
	wsCard := cardStyle.Render(
		labelStyle.Render("Workspace") + "\n" +
			valueStyle.Render("  "+wsInfo),
	)

	serversCard := cardStyle.Render(
		labelStyle.Render("Servers") + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d connected", d.serverCount)) + "\n" +
			valueStyle.Render(fmt.Sprintf("  %d roles loaded", d.roleCount)),
	)

	daemonStatus := errorStyle.Render("● Stopped")
	if d.daemonRunning {
		daemonStatus = successStyle.Render("● Running")
	}
	daemonCard := cardStyle.Render(
		labelStyle.Render("Daemon") + "\n" +
			"  " + daemonStatus,
	)

	if useHorizontal {
		row := lipgloss.JoinHorizontal(lipgloss.Top, wsCard, " ", serversCard, " ", daemonCard)
		b.WriteString(row)
	} else {
		b.WriteString(wsCard)
		b.WriteString("\n")
		b.WriteString(serversCard)
		b.WriteString("\n")
		b.WriteString(daemonCard)
	}
	b.WriteString("\n\n")

	b.WriteString(subtitleStyle.Render("  Loaded Roles"))
	b.WriteString("\n")
	if d.ctx.ServerEngine != nil {
		roles := d.ctx.ServerEngine.GetLoadedRoles()
		if len(roles) == 0 {
			b.WriteString(hintStyle.Render("  No roles loaded. Go to Servers tab → [l] to load roles."))
		} else {
			for _, r := range roles {
				b.WriteString(fmt.Sprintf("  %s-%s  [%s] mode=%s\n",
					labelStyle.Render(r.ServerName),
					valueStyle.Render(r.RoleName),
					r.APIType, r.MessageMode))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  [Tab] Switch tabs  [/] Command  [?] Help"))

	return b.String()
}
