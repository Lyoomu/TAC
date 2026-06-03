package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

type rolesViewModel struct {
	ctx        *AppContext
	width      int
	height     int
	items      []datamodel.Role
	cursor     int
	form       *formModel
	selector   *selectorModel
	detailMode bool

	importItems []datamodel.Role
}

func newRolesViewModel(ctx *AppContext) *rolesViewModel {
	return &rolesViewModel{ctx: ctx, form: newFormModel(), selector: newSelectorModel()}
}

func (v *rolesViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *rolesViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return rolesDataMsg{}
	}
}

type rolesDataMsg struct{}

func (v *rolesViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case rolesDataMsg:
		if v.ctx.RoleEngine != nil {
			if list, err := v.ctx.RoleEngine.List(); err == nil {
				v.items = list
			}
		}
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

		if v.selector.IsActive() {
			done, cancelled := v.selector.Update(msg)
			if cancelled {
				v.importItems = nil
				return nil
			}
			if done {
				return v.handleSelectorDone()
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
		case "t", "T":
			if v.cursor < len(v.items) {
				return func() tea.Msg {
					return switchToChatMsg{roleName: v.items[v.cursor].Name}
				}
			}
		case "enter":
			if v.cursor < len(v.items) {
				v.detailMode = !v.detailMode
			}
		case "c", "C":
			compNames := v.getComponentNames()
			toolNames := v.getToolNames()
			modelNames := v.getModelNames()
			v.form.startCreate("Create Role", []formField{
				{label: "Name", placeholder: "unique role name", required: true},
				{label: "Description", placeholder: "role description", required: false},
				{label: "Components", multiSelect: true, multiSelectItems: compNames},
				{label: "Tools", multiSelect: true, multiSelectItems: toolNames},
				{label: "Message Mode", required: false, options: []string{
					"interrupt", "queue", "reject",
				}},
				{label: "Model", required: true, options: modelNames},
			})
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				r := v.items[v.cursor]
				mode := r.MessageMode
				if mode == "" {
					mode = "interrupt"
				}
				compNames := v.getComponentNames()
				toolNames := v.getToolNames()
				modelNames := v.getModelNames()
				v.form.startEdit("Edit Role: "+r.Name, []formField{
					{label: "Description", placeholder: "role description", value: r.Description, required: false},
					{label: "Components", multiSelect: true, multiSelectItems: compNames, value: strings.Join(r.Components, ",")},
					{label: "Tools", multiSelect: true, multiSelectItems: toolNames, value: strings.Join(r.Tools, ",")},
					{label: "Message Mode", value: mode, required: false, options: []string{
						"interrupt", "queue", "reject",
					}},
					{label: "Model", value: r.Model, required: true, options: modelNames},
				})
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) && v.ctx.RoleEngine != nil {
				name := v.items[v.cursor].Name
				if err := v.ctx.RoleEngine.Delete(name); err != nil {
					return func() tea.Msg { return statusMsg("Delete failed: " + err.Error()) }
				}
				if list, err := v.ctx.RoleEngine.List(); err == nil {
					v.items = list
					if v.cursor >= len(v.items) {
						v.cursor = len(v.items) - 1
						if v.cursor < 0 {
							v.cursor = 0
						}
					}
				}
				return func() tea.Msg { return statusMsg("Deleted role: " + name) }
			}
		case "i", "I":
			items, err := scanImportRoles()
			if err != nil {
				return func() tea.Msg { return statusMsg("Scan import failed: " + err.Error()) }
			}
			if len(items) == 0 {
				return func() tea.Msg { return statusMsg("No role files found in Input/Roles") }
			}
			v.importItems = items
			var names []string
			for _, r := range items {
				names = append(names, r.Name)
			}
			v.selector.start(selectorImport, "Select Roles to Import", names)
			return nil
		case "o", "O":
			if len(v.items) == 0 {
				return func() tea.Msg { return statusMsg("No roles to export") }
			}
			var names []string
			for _, r := range v.items {
				names = append(names, r.Name)
			}
			v.selector.start(selectorExport, "Select Roles to Export", names)
			return nil
		case "esc":
			v.detailMode = false
		}
	}
	return nil
}

func (v *rolesViewModel) handleSelectorDone() tea.Cmd {
	selected := v.selector.SelectedIndices()
	if len(selected) == 0 {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("No items selected") }
	}
	if v.selector.mode == selectorExport {
		if err := exportRoles(v.items, selected); err != nil {
			return func() tea.Msg { return statusMsg("Export failed: " + err.Error()) }
		}
		_, outDir := importExportDirs()
		return func() tea.Msg {
			return statusMsg(fmt.Sprintf("Exported %d role(s) to %s", len(selected), filepath.Join(outDir, "Roles")))
		}
	}

	if v.ctx.RoleEngine == nil {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("RoleEngine not available") }
	}
	var imported int
	for _, idx := range selected {
		if idx < 0 || idx >= len(v.importItems) {
			continue
		}
		r := v.importItems[idx]
		if err := v.ctx.RoleEngine.Create(&r); err != nil {
			continue
		}
		imported++
	}
	v.importItems = nil
	if list, err := v.ctx.RoleEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg(fmt.Sprintf("Imported %d role(s)", imported)) }
}

func (v *rolesViewModel) getModelNames() []string {
	var names []string
	if v.ctx.ModelsEngine != nil {
		if list, err := v.ctx.ModelsEngine.List(); err == nil {
			for _, m := range list {
				names = append(names, m.Name)
			}
		}
	}
	return names
}

func (v *rolesViewModel) getComponentNames() []string {
	var names []string
	if v.ctx.ComponentEngine != nil {
		if list, err := v.ctx.ComponentEngine.List(); err == nil {
			for _, c := range list {
				names = append(names, c.Name)
			}
		}
	}
	return names
}

func (v *rolesViewModel) getToolNames() []string {
	var names []string
	if v.ctx.ToolEngine != nil {
		for _, t := range v.ctx.ToolEngine.List() {
			names = append(names, t.Name)
		}
	}
	return names
}

func (v *rolesViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()

	if v.form.mode == formModeCreate {

		if v.ctx.RoleEngine == nil {
			return func() tea.Msg { return statusMsg("RoleEngine not available") }
		}

		var comps []string
		if vals[2] != "" {
			for _, c := range strings.Split(vals[2], ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					comps = append(comps, c)
				}
			}
		}

		var tools []string
		if vals[3] != "" {
			for _, t := range strings.Split(vals[3], ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tools = append(tools, t)
				}
			}
		}

		msgMode := vals[4]
		if msgMode == "" {
			msgMode = "interrupt"
		}

		r := &datamodel.Role{
			Name:        vals[0],
			Description: vals[1],
			Components:  comps,
			Tools:       tools,
			MessageMode: msgMode,
			Model:       vals[5],
		}
		if err := v.ctx.RoleEngine.Create(r); err != nil {
			return func() tea.Msg { return statusMsg("Create failed: " + err.Error()) }
		}

		updated := false
		for i := range v.items {
			if v.items[i].Name == r.Name {
				v.items[i] = *r
				updated = true
				break
			}
		}
		if !updated {
			v.items = append(v.items, *r)
		}
		if list, err := v.ctx.RoleEngine.List(); err == nil {
			v.items = list
		}
		return func() tea.Msg { return statusMsg("Created role: " + r.Name) }
	}

	if v.ctx.RoleEngine == nil || v.cursor >= len(v.items) {
		return func() tea.Msg { return statusMsg("RoleEngine not available") }
	}

	name := v.items[v.cursor].Name

	var comps []string
	if vals[1] != "" {
		for _, c := range strings.Split(vals[1], ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				comps = append(comps, c)
			}
		}
	}

	var tools []string
	if vals[2] != "" {
		for _, t := range strings.Split(vals[2], ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tools = append(tools, t)
			}
		}
	}

	updates := &datamodel.Role{
		Description: vals[0],
		Components:  comps,
		Tools:       tools,
		MessageMode: vals[3],
		Model:       vals[4],
	}
	if err := v.ctx.RoleEngine.Update(name, updates); err != nil {
		return func() tea.Msg { return statusMsg("Update failed: " + err.Error()) }
	}

	if v.cursor < len(v.items) {
		if updates.Description != "" {
			v.items[v.cursor].Description = updates.Description
		}
		if updates.Components != nil {
			v.items[v.cursor].Components = updates.Components
		}
		if updates.Tools != nil {
			v.items[v.cursor].Tools = updates.Tools
		}
		if updates.MessageMode != "" {
			v.items[v.cursor].MessageMode = updates.MessageMode
		}
		if updates.Model != "" {
			v.items[v.cursor].Model = updates.Model
		}
	}
	if list, err := v.ctx.RoleEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg("Updated role: " + name) }
}

func (v *rolesViewModel) View() string {

	if v.selector.IsActive() {
		return v.selector.View()
	}

	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Roles"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No roles defined. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-18s %-12s %-18s %-20s %-20s", "NAME", "MODE", "MODEL", "COMPONENTS", "TOOLS")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	for i, r := range v.items {
		comps := truncate(strings.Join(r.Components, ","), 20)
		tools := truncate(strings.Join(r.Tools, ","), 20)
		mode := r.MessageMode
		if mode == "" {
			mode = "interrupt"
		}
		modelName := r.Model
		if modelName == "" {
			modelName = "(none)"
		}
		row := fmt.Sprintf("  %-18s %-12s %-18s %-20s %-20s",
			truncate(r.Name, 18), truncate(mode, 12), truncate(modelName, 18), comps, tools)
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if v.detailMode && v.cursor < len(v.items) {
		r := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Role Details"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Name:       %s", r.Name)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Mode:       %s", r.MessageMode)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Model:      %s", r.Model)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Components: %s", strings.Join(r.Components, ", "))))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Tools:      %s", strings.Join(r.Tools, ", "))))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Desc:       %s", r.Description)))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Details  [t] Chat  [c] Create  [e] Edit  [d] Delete  [i] Import  [o] Export"))
	}

	return b.String()
}
