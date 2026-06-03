package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
	"github.com/Lyoomu/TAC/Trigger/internal/model"
	pb "github.com/Lyoomu/TAC/proto"
	"gopkg.in/yaml.v3"
)

type eventItem struct {
	ID          string
	Name        string
	Description string
	RoleKey     string
	InitialMsg  string
	SessionMode string
	MessageMode string
	EnvPreset   string
	EnvValue    string
	EnvOverrideCount int32
	EnvCount    int32
}

type eventsViewModel struct {
	ctx          *AppContext
	width        int
	height       int
	items        []eventItem
	cursor       int
	scrollOffset int
	detailMode   bool
	form         *formModel
}

func newEventsViewModel(ctx *AppContext) *eventsViewModel {
	return &eventsViewModel{ctx: ctx, form: newFormModel()}
}

func (v *eventsViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *eventsViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return eventsDataMsg{}
	}
}

type eventsDataMsg struct{}

func (v *eventsViewModel) Update(msg tea.Msg) tea.Cmd {

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
	case eventsDataMsg:
		if !daemon.IsDaemonRunning() {
			v.items = nil
			return nil
		}
		client, err := daemon.NewClient()
		if err != nil {
			return nil
		}
		resp, err := client.ListEvents()
		if err != nil {
			return nil
		}
		v.items = make([]eventItem, 0, len(resp.Events))
		for _, e := range resp.Events {
			desc, initialMsg, messageMode, envPreset, env := v.loadEventDetails(e.Id)
			v.items = append(v.items, eventItem{
				ID:          e.Id,
				Name:        e.Name,
				Description: desc,
				RoleKey:     e.RoleKey,
				InitialMsg:  initialMsg,
				SessionMode: e.SessionMode,
				MessageMode: messageMode,
				EnvPreset:   envPreset,
				EnvValue:    envMapToString(env),
				EnvOverrideCount: int32(len(env)),
				EnvCount:    e.EnvCount,
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
			roleOptions := v.roleKeyOptions("")
			if len(roleOptions) == 0 {
				return func() tea.Msg {
					return triggerStatusMsg("No roles loaded. Go to Servers tab -> [l] to load roles.")
				}
			}
			envPresetOptions := v.envPresetOptions("")
			v.form.startCreate("Create Event", []formField{
				{label: "Name", placeholder: "event name", required: true},
				{label: "Description", placeholder: "description", required: false},
				{label: "Role Key", placeholder: "ServerName-RoleName", required: true, options: roleOptions},
				{label: "Initial Msg", placeholder: "initial message template", required: false},
				{label: "Session Mode", required: true, options: []string{"shared", "new"}},
				{label: "Message Mode", required: false, options: []string{"role option", "queue", "reject", "interrupt"}},
				{label: "Env Preset", required: false, options: envPresetOptions},
				{label: "Env Override", placeholder: "KEY=VALUE,KEY2=file:path", required: false},
			})
			v.detailMode = false
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				e := v.items[v.cursor]
				roleOptions := v.roleKeyOptions(e.RoleKey)
				if len(roleOptions) == 0 {
					return func() tea.Msg {
						return triggerStatusMsg("No roles loaded. Go to Servers tab -> [l] to load roles.")
					}
				}
				envPresetOptions := v.envPresetOptions(e.EnvPreset)
				v.form.startEdit("Edit Event: "+e.Name, []formField{
					{label: "Name", placeholder: "event name", value: e.Name, required: true},
					{label: "Description", placeholder: "description", value: e.Description, required: false},
					{label: "Role Key", placeholder: "ServerName-RoleName", value: e.RoleKey, required: true, options: roleOptions},
					{label: "Initial Msg", placeholder: "initial message template", value: e.InitialMsg, required: false},
					{label: "Session Mode", value: e.SessionMode, required: true, options: []string{"shared", "new"}},
					{label: "Message Mode", value: messageModeDisplay(e.MessageMode), required: false, options: []string{"role option", "queue", "reject", "interrupt"}},
					{label: "Env Preset", value: envPresetDisplay(e.EnvPreset), required: false, options: envPresetOptions},
					{label: "Env Override", placeholder: "KEY=VALUE,KEY2=file:path", value: e.EnvValue, required: false},
				})
				v.detailMode = false
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) {
				id := v.items[v.cursor].ID
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.DeleteEvent(id)
					}
					return eventsDataMsg{}
				}
			}
		}
	}
	return nil
}

func (v *eventsViewModel) roleKeyOptions(current string) []string {
	if v.ctx == nil || v.ctx.ServerEngine == nil {
		if current == "" {
			return nil
		}
		return []string{current}
	}

	roles := v.ctx.ServerEngine.GetLoadedRoles()
	if len(roles) == 0 {
		if current == "" {
			return nil
		}
		return []string{current}
	}

	seen := make(map[string]struct{}, len(roles))
	options := make([]string, 0, len(roles)+1)
	for _, r := range roles {
		key := r.ServerName + "-" + r.RoleName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		options = append(options, key)
	}
	if current != "" {
		if _, ok := seen[current]; !ok {
			options = append(options, current)
		}
	}

	sort.Strings(options)
	return options
}

func (v *eventsViewModel) envPresetOptions(current string) []string {
	options := []string{"none"}
	if !daemon.IsDaemonRunning() {
		if current != "" && current != "none" {
			options = append(options, current)
		}
		return options
	}

	client, err := daemon.NewClient()
	if err != nil {
		if current != "" && current != "none" {
			options = append(options, current)
		}
		return options
	}
	resp, err := client.ListEnvPresets()
	if err != nil {
		if current != "" && current != "none" {
			options = append(options, current)
		}
		return options
	}

	var names []string
	for _, p := range resp.Presets {
		if p.Name == "" {
			continue
		}
		names = append(names, p.Name)
	}
	if current != "" && current != "none" && !containsString(names, current) {
		names = append(names, current)
	}
	sort.Strings(names)
	options = append(options, names...)
	return options
}

func (v *eventsViewModel) loadEventDetails(eventID string) (string, string, string, string, map[string]string) {
	if v.ctx == nil || v.ctx.WorkspaceEngine == nil {
		return "", "", "", "", nil
	}

	tacPath, err := v.ctx.WorkspaceEngine.GetActiveTACPath()
	if err != nil {
		return "", "", "", "", nil
	}

	path := filepath.Join(tacPath, "events", eventID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", "", nil
	}

	var ev model.Event
	if err := yaml.Unmarshal(data, &ev); err != nil {
		return "", "", "", "", nil
	}

	return ev.Description, ev.InitialMsg, ev.MessageMode, ev.EnvPreset, ev.Env
}

func messageModeDisplay(mode string) string {
	if mode == "" {
		return "role option"
	}
	return mode
}

func envPresetDisplay(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func normalizeEnvPreset(value string) string {
	if value == "none" {
		return ""
	}
	return value
}

func normalizeMessageMode(value string) string {
	if value == "role option" {
		return ""
	}
	return value
}


func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (v *eventsViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()
	mode := v.form.mode

	msgMode := normalizeMessageMode(vals[5])
	envPreset := normalizeEnvPreset(vals[6])
	env := parseEnvString(vals[7])
	if mode == formModeCreate {
		return func() tea.Msg {
			if client, err := daemon.NewClient(); err == nil {
				req := &pb.CreateEventRequest{
					Name:        vals[0],
					Description: vals[1],
					RoleKey:     vals[2],
					InitialMsg:  vals[3],
					SessionMode: vals[4],
					MessageMode: msgMode,
					EnvPreset:   envPreset,
					Env:         env,
				}
				_, _ = client.CreateEvent(req)
			}
			return eventsDataMsg{}
		}
	}

	if v.cursor < len(v.items) {
		id := v.items[v.cursor].ID
		return func() tea.Msg {
			if client, err := daemon.NewClient(); err == nil {
				req := &pb.UpdateEventRequest{
					Id:          id,
					Name:        vals[0],
					Description: vals[1],
					RoleKey:     vals[2],
					InitialMsg:  vals[3],
					SessionMode: vals[4],
					MessageMode: msgMode,
					EnvPreset:     envPreset,
					Env:           env,
					SetMessageMode: true,
					SetEnvPreset:   true,
					SetEnv:         true,
				}
				_, _ = client.UpdateEvent(req)
			}
			return eventsDataMsg{}
		}
	}
	return nil
}

func (v *eventsViewModel) ensureCursorVisible() {
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

func (v *eventsViewModel) View() string {
	if v.form.IsActive() {
		return v.form.View()
	}
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Events"))
	b.WriteString("\n\n")

	if !daemon.IsDaemonRunning() {
		b.WriteString(errorStyle.Render("  Daemon is not running."))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  Use 'daemon start' to launch the background daemon."))
		return b.String()
	}

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No events defined. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-16s %-18s %-25s %-12s %-6s", "ID", "NAME", "ROLE_KEY", "SESSION", "ENV")
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
		e := v.items[i]
		row := fmt.Sprintf("  %-16s %-18s %-25s %-12s %-6d",
			truncate(e.ID, 16), truncate(e.Name, 18), truncate(e.RoleKey, 25), e.SessionMode, e.EnvCount)
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
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
	if v.detailMode && v.cursor < len(v.items) {
		e := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Event Details"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  ID:      %s", e.ID)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Name:    %s", e.Name)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Role:    %s", e.RoleKey)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Session: %s", e.SessionMode)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Env:     %d (preset=%s, override=%d)", e.EnvCount, envPresetDisplay(e.EnvPreset), e.EnvOverrideCount)))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [\u2191/\u2193] Navigate  [Enter] Details  [c] Create  [e] Edit  [d] Delete"))
	}

	return b.String()
}
