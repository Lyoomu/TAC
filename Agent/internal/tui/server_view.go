package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type serverViewModel struct {
	ctx       *AppContext
	width     int
	height    int
	isRunning bool
	addr      string
}

func newServerViewModel(ctx *AppContext) *serverViewModel {
	return &serverViewModel{ctx: ctx}
}

func (v *serverViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *serverViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return serverDataMsg{}
	}
}

type serverDataMsg struct{}

func (v *serverViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "s", "S":
			if v.ctx.Server == nil {
				return func() tea.Msg { return statusMsg("Server not initialized") }
			}
			if v.isRunning {
				return func() tea.Msg { return statusMsg("Server already running at " + v.addr) }
			}
			if err := v.ctx.Server.SetupTLS("", ""); err != nil {
				return func() tea.Msg { return statusMsg("TLS setup failed: " + err.Error()) }
			}
			if err := v.ctx.Server.Start(":50051"); err != nil {
				return func() tea.Msg { return statusMsg("Start failed: " + err.Error()) }
			}
			v.isRunning = true
			v.addr = v.ctx.Server.Addr()
			return func() tea.Msg { return statusMsg("Server started on " + v.addr) }

		case "x", "X":
			if v.ctx.Server == nil || !v.isRunning {
				return func() tea.Msg { return statusMsg("Server not running") }
			}
			v.ctx.Server.Stop()
			v.isRunning = false
			v.addr = ""
			return func() tea.Msg { return statusMsg("Server stopped") }
		}

	case serverDataMsg:

		if v.ctx.Server != nil && v.ctx.Server.Addr() != "" {
			v.isRunning = true
			v.addr = v.ctx.Server.Addr()
		}
	}
	return nil
}

func (v *serverViewModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Server Status"))
	b.WriteString("\n\n")

	if v.isRunning {
		b.WriteString(successStyle.Render("  ● Running"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render("  Address: " + v.addr))
	} else {
		b.WriteString(hintStyle.Render("  ○ Not running"))
		b.WriteString("\n\n")
		b.WriteString(valueStyle.Render("  Use 'server start --addr :50051' to launch the gRPC server."))
	}

	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Configuration"))
	b.WriteString("\n")
	if v.ctx.Config != nil {
		b.WriteString(valueStyle.Render("  DB Path:   " + v.ctx.Config.WorkPath.DB))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render("  Tool Path: " + v.ctx.Config.WorkPath.Tool))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("  [s] Start Server  [x] Stop Server"))

	return b.String()
}
