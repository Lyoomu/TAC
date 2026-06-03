package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	"github.com/Lyoomu/TAC/Trigger/internal/workspace"
)

type workspaceViewModel struct {
	ctx          *AppContext
	width        int
	height       int
	items        []model.Workspace
	cursor       int
	scrollOffset int
	form         *formModel
}

func newWorkspaceViewModel(ctx *AppContext) *workspaceViewModel {
	return &workspaceViewModel{ctx: ctx, form: newFormModel()}
}

func (v *workspaceViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *workspaceViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return workspaceDataMsg{}
	}
}

type workspaceDataMsg struct{}

func (v *workspaceViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case workspaceDataMsg:
		v.reload()
	case tea.KeyMsg:
		if v.form.IsActive() {
			done, cancelled := v.form.Update(msg)
			if cancelled {
				return nil
			}
			if done {
				return v.handleFormDone()
			}
			return nil
		}

		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.items)-1 {
				v.cursor++
			}
		case "b", "B":
			v.form.startCreate("Bind Workspace", []formField{
				{label: "Name", placeholder: "logical workspace name", required: true},
				{label: "Path", placeholder: "disk directory path", value: ".", required: true},
			})
			return nil
		case "a", "A", "enter":
			if v.cursor < len(v.items) {
				ws := v.items[v.cursor]
				if v.ctx.WorkspaceEngine == nil {
					return func() tea.Msg { return triggerStatusMsg("WorkspaceEngine not available") }
				}
				if err := v.ctx.WorkspaceEngine.Activate(ws.Name); err != nil {
					return func() tea.Msg { return triggerStatusMsg("Activate failed: " + err.Error()) }
				}
				v.reload()
				return func() tea.Msg { return triggerStatusMsg("Activated: " + ws.Name) }
			}
		case "d", "D":
			if v.cursor < len(v.items) {
				ws := v.items[v.cursor]
				if v.ctx.WorkspaceEngine == nil {
					return func() tea.Msg { return triggerStatusMsg("WorkspaceEngine not available") }
				}
				if err := v.ctx.WorkspaceEngine.Unbind(ws.Name); err != nil {
					return func() tea.Msg { return triggerStatusMsg("Unbind failed: " + err.Error()) }
				}
				v.reload()
				return func() tea.Msg { return triggerStatusMsg("Unbound: " + ws.Name) }
			}
		}
	}
	return nil
}

func (v *workspaceViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()
	if v.ctx.WorkspaceEngine == nil {
		return func() tea.Msg { return triggerStatusMsg("WorkspaceEngine not available") }
	}

	name := vals[0]
	path := vals[1]

	if err := v.ctx.WorkspaceEngine.Bind(name, path); err != nil {
		return func() tea.Msg { return triggerStatusMsg("Bind failed: " + workspaceErrorMessage(err)) }
	}
	v.reload()
	return func() tea.Msg { return triggerStatusMsg(fmt.Sprintf("Bound: %s -> %s", name, path)) }
}

func (v *workspaceViewModel) reload() {
	if v.ctx.WorkspaceEngine != nil {
		v.items = v.ctx.WorkspaceEngine.List()
	}
	if v.cursor >= len(v.items) {
		v.cursor = len(v.items) - 1
		if v.cursor < 0 {
			v.cursor = 0
		}
	}
}

func (v *workspaceViewModel) ensureCursorVisible() {
	maxVisible := v.height - 6
	if maxVisible < 3 {
		maxVisible = 3
	}
	if v.cursor < v.scrollOffset {
		v.scrollOffset = v.cursor
	}
	if v.cursor >= v.scrollOffset+maxVisible {
		v.scrollOffset = v.cursor - maxVisible + 1
	}
}

func (v *workspaceViewModel) View() string {
	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("  Workspaces"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No workspaces bound. Press [b] to bind one."))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [b] Bind  [/] Command  [Tab] Switch tabs"))
		return b.String()
	}

	nameWidth := 20
	pathWidth := v.width - nameWidth - 16
	if pathWidth < 24 {
		pathWidth = 24
	}

	header := fmt.Sprintf("  %-*s  %-*s  %s", nameWidth, "NAME", pathWidth, "PATH", "ACTIVE")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	// Calculate visible window
	maxVisible := v.height - 6
	if maxVisible < 3 {
		maxVisible = 3
	}
	v.ensureCursorVisible()
	end := v.scrollOffset + maxVisible
	if end > len(v.items) {
		end = len(v.items)
	}

	for i := v.scrollOffset; i < end; i++ {
		ws := v.items[i]
		active := ""
		if ws.IsActive {
			active = "*"
		}
		line := fmt.Sprintf("  %-*s  %-*s  %s", nameWidth, truncate(ws.Name, nameWidth), pathWidth, truncate(ws.Path, pathWidth), active)
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(line))
		} else {
			b.WriteString(tableRowStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(v.items) > maxVisible {
		scrollInfo := fmt.Sprintf("  (%d-%d of %d)", v.scrollOffset+1, end, len(v.items))
		b.WriteString(hintStyle.Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  [\u2191/\u2193] Navigate  [Enter/a] Activate  [b] Bind  [d] Unbind"))
	return b.String()
}

func workspaceErrorMessage(err error) string {
	switch err {
	case workspace.ErrWorkspaceExists:
		return "workspace already exists"
	case workspace.ErrPathAlreadyBound:
		return "path already bound to another workspace"
	case workspace.ErrInvalidName:
		return "invalid workspace name"
	case workspace.ErrInvalidPath:
		return "invalid workspace path"
	default:
		return err.Error()
	}
}
