package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
)

type daemonViewModel struct {
	ctx          *AppContext
	width        int
	height       int
	running      bool
	workspace    string
	triggerCount int32
	eventCount   int32
	runningCount int32
	uptime       string
}

func newDaemonViewModel(ctx *AppContext) *daemonViewModel {
	return &daemonViewModel{ctx: ctx}
}

func (v *daemonViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *daemonViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return daemonDataMsg{}
	}
}

type daemonDataMsg struct{}

func (v *daemonViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case daemonDataMsg:
		v.running = daemon.IsDaemonRunning()
		if v.running {
			client, err := daemon.NewClient()
			if err != nil {
				return nil
			}
			status, err := client.GetDaemonStatus()
			if err != nil {
				return nil
			}
			v.workspace = status.Workspace
			v.triggerCount = status.TriggerCount
			v.eventCount = status.EventCount
			v.runningCount = status.RunningTriggerCount
			v.uptime = status.Uptime
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "s", "S":
			if v.ctx.CommandFunc != nil {
				out, err := v.ctx.CommandFunc("daemon start")
				if err != nil {
					return func() tea.Msg { return triggerStatusMsg("Start failed: " + err.Error()) }
				}
				return func() tea.Msg { return triggerStatusMsg(out) }
			}
			return func() tea.Msg { return triggerStatusMsg("CommandFunc not available") }
		case "x", "X":
			if v.ctx.CommandFunc != nil {
				out, err := v.ctx.CommandFunc("daemon stop")
				if err != nil {
					return func() tea.Msg { return triggerStatusMsg("Stop failed: " + err.Error()) }
				}
				return func() tea.Msg { return triggerStatusMsg(out) }
			}
			return func() tea.Msg { return triggerStatusMsg("CommandFunc not available") }
		case "r", "R":
			return v.refresh()
		}
	}
	return nil
}

func (v *daemonViewModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Daemon Status"))
	b.WriteString("\n\n")

	if !v.running {
		b.WriteString(errorStyle.Render("  ● Daemon is not running"))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  Press [s] to start the daemon or use 'daemon start' command."))
		return b.String()
	}

	b.WriteString(successStyle.Render("  ● Daemon is running"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("  Workspace:      "))
	b.WriteString(valueStyle.Render(v.workspace))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("  Uptime:         "))
	b.WriteString(valueStyle.Render(v.uptime))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("  Triggers:       "))
	b.WriteString(valueStyle.Render(strings.TrimSpace(
		strings.Join([]string{
			itoa(v.triggerCount) + " total",
			itoa(v.runningCount) + " running",
		}, ", "),
	)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("  Events:         "))
	b.WriteString(valueStyle.Render(itoa(v.eventCount)))
	b.WriteString("\n\n")

	b.WriteString(hintStyle.Render("  [s] Start  [x] Stop  [r] Refresh"))

	return b.String()
}

func itoa(v int32) string {
	return strings.TrimSpace(fmt.Sprintf("%d", v))
}
