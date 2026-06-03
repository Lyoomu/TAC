package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
	pb "github.com/Lyoomu/TAC/proto"
)

type envPresetItem struct {
	Name        string
	Description string
	EnvValue    string
	EnvCount    int32
}

type envPresetsViewModel struct {
	ctx          *AppContext
	width        int
	height       int
	items        []envPresetItem
	cursor       int
	scrollOffset int
	detailMode   bool
	form         *formModel
	editingName  string
}

func newEnvPresetsViewModel(ctx *AppContext) *envPresetsViewModel {
	return &envPresetsViewModel{ctx: ctx, form: newFormModel()}
}

func (v *envPresetsViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *envPresetsViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return envPresetsDataMsg{}
	}
}

type envPresetsDataMsg struct{}

func (v *envPresetsViewModel) Update(msg tea.Msg) tea.Cmd {
	if v.form.IsActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			done, cancelled := v.form.Update(msg)
			if cancelled {
				return nil
			}
			if done {
				return v.handleFormDone()
			}
		}
		return nil
	}

	switch msg := msg.(type) {
	case envPresetsDataMsg:
		if !daemon.IsDaemonRunning() {
			v.items = nil
			return nil
		}
		client, err := daemon.NewClient()
		if err != nil {
			return nil
		}
		resp, err := client.ListEnvPresets()
		if err != nil {
			return nil
		}
		v.items = make([]envPresetItem, 0, len(resp.Presets))
		for _, p := range resp.Presets {
			v.items = append(v.items, envPresetItem{
				Name:        p.Name,
				Description: p.Description,
				EnvValue:    envMapToString(p.Env),
				EnvCount:    int32(len(p.Env)),
			})
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
			v.detailMode = false
		case "down", "j":
			if v.cursor < len(v.items)-1 {
				v.cursor++
			}
			v.detailMode = false
		case "enter":
			if v.cursor < len(v.items) {
				v.detailMode = !v.detailMode
			}
		case "esc":
			v.detailMode = false
		case "c", "C":
			v.editingName = ""
			v.form.startCreate("Create Env Preset", []formField{
				{label: "Name", placeholder: "preset name", required: true},
				{label: "Description", placeholder: "description", required: false},
				{label: "Env", placeholder: "KEY=VALUE,KEY2=file:path", required: false},
			})
			v.detailMode = false
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				p := v.items[v.cursor]
				v.editingName = p.Name
				v.form.startEdit("Edit Env Preset: "+p.Name, []formField{
					{label: "Description", placeholder: "description", value: p.Description, required: false},
					{label: "Env", placeholder: "KEY=VALUE,KEY2=file:path", value: p.EnvValue, required: false},
				})
				v.detailMode = false
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) {
				name := v.items[v.cursor].Name
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.DeleteEnvPreset(name)
					}
					return envPresetsDataMsg{}
				}
			}
		}
	}
	return nil
}

func (v *envPresetsViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()
	mode := v.form.mode

	if mode == formModeCreate {
		return func() tea.Msg {
			if client, err := daemon.NewClient(); err == nil {
				req := &pb.CreateEnvPresetRequest{
					Name:        vals[0],
					Description: vals[1],
					Env:         parseEnvString(vals[2]),
				}
				_, _ = client.CreateEnvPreset(req)
			}
			return envPresetsDataMsg{}
		}
	}

	name := v.editingName
	return func() tea.Msg {
		if client, err := daemon.NewClient(); err == nil {
			req := &pb.UpdateEnvPresetRequest{
				Name:           name,
				Description:    vals[0],
				Env:            parseEnvString(vals[1]),
				SetDescription: true,
				SetEnv:         true,
			}
			_, _ = client.UpdateEnvPreset(req)
		}
		return envPresetsDataMsg{}
	}
}

func (v *envPresetsViewModel) ensureCursorVisible() {
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

func (v *envPresetsViewModel) View() string {
	if v.form.IsActive() {
		return v.form.View()
	}
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Env Presets"))
	b.WriteString("\n\n")

	if !daemon.IsDaemonRunning() {
		b.WriteString(errorStyle.Render("  Daemon is not running."))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  Use 'daemon start' to launch the background daemon."))
		return b.String()
	}

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No env presets defined. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-18s %-6s %-30s", "NAME", "ENV", "DESCRIPTION")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

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
		p := v.items[i]
		row := fmt.Sprintf("  %-18s %-6d %-30s",
			truncate(p.Name, 18), p.EnvCount, truncate(p.Description, 30))
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	if len(v.items) > maxVisible {
		scrollInfo := fmt.Sprintf("  (%d-%d of %d)", v.scrollOffset+1, end, len(v.items))
		b.WriteString(hintStyle.Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if v.detailMode && v.cursor < len(v.items) {
		p := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Env Preset Details"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Name: %s", p.Name)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Description: %s", p.Description)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Env: %d", p.EnvCount)))
		b.WriteString("\n")
		if p.EnvValue != "" {
			b.WriteString(valueStyle.Render(fmt.Sprintf("  Values: %s", truncate(p.EnvValue, 60))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Details  [c] Create  [e] Edit  [d] Delete"))
	}

	return b.String()
}
