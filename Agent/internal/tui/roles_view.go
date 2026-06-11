package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

type rolesViewModel struct {
	ctx              *AppContext
	width            int
	height           int
	items            []datamodel.Role
	cursor           int
	form             *formModel
	selector         *selectorModel
	conflictResolver *conflictResolverModel
	detailMode       bool

	importItems   []datamodel.Role
	pendingImport []datamodel.Role
}

func newRolesViewModel(ctx *AppContext) *rolesViewModel {
	return &rolesViewModel{
		ctx:              ctx,
		form:             newFormModel(),
		selector:         newSelectorModel(),
		conflictResolver: newConflictResolverModel(),
	}
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

		if v.conflictResolver.IsActive() {
			done, cancelled := v.conflictResolver.Update(msg)
			if cancelled {
				v.pendingImport = nil
				return nil
			}
			if done {
				return v.executeRoleImport()
			}
			return nil
		}

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
				{label: "API Type", required: true, options: []string{
					string(datamodel.APITypeChatCompletion),
					string(datamodel.APITypeResponses),
					string(datamodel.APITypeAnthropic),
				}},
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
				apiType := string(r.APIType)
				if apiType == "" {
					apiType = string(datamodel.APITypeChatCompletion)
				}
				compNames := v.getComponentNames()
				toolNames := v.getToolNames()
				modelNames := v.getModelNames()
				v.form.startEdit("Edit Role: "+r.Name, []formField{
					{label: "Description", placeholder: "role description", value: r.Description, required: false},
					{label: "API Type", value: apiType, required: true, options: []string{
						string(datamodel.APITypeChatCompletion),
						string(datamodel.APITypeResponses),
						string(datamodel.APITypeAnthropic),
					}},
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
				return func() tea.Msg { return statusMsg("No role files found in Import/Roles") }
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

	var toImport []datamodel.Role
	for _, idx := range selected {
		if idx >= 0 && idx < len(v.importItems) {
			toImport = append(toImport, v.importItems[idx])
		}
	}

	conflicts := v.detectRoleConflicts(toImport)
	if len(conflicts) > 0 {
		v.pendingImport = toImport
		existingNames := v.buildRoleNameSet()
		v.conflictResolver.start("Role Import Conflicts", conflicts, existingNames)
		return nil
	}

	return v.doImportRoles(toImport)
}

func (v *rolesViewModel) detectRoleConflicts(toImport []datamodel.Role) []*conflictEntry {
	existing := make(map[string]datamodel.Role)
	for _, r := range v.items {
		existing[r.Name] = r
	}

	var conflicts []*conflictEntry
	for _, r := range toImport {
		if ex, ok := existing[r.Name]; ok {
			info := fmt.Sprintf("role, model: %s", ex.Model)
			conflicts = append(conflicts, &conflictEntry{
				Name:         r.Name,
				ResourceType: "role",
				ExistingInfo: info,
			})
		}
	}
	return conflicts
}

func (v *rolesViewModel) buildRoleNameSet() map[string]bool {
	names := make(map[string]bool)
	for _, r := range v.items {
		names[r.Name] = true
	}
	return names
}

func (v *rolesViewModel) executeRoleImport() tea.Cmd {
	resolved := v.conflictResolver.GetResolved()
	v.conflictResolver.cancel()

	toImport := v.pendingImport
	v.pendingImport = nil
	v.importItems = nil

	var imported, overwritten, renamed, skipped int
	for _, r := range toImport {
		if entry, ok := resolved[r.Name]; ok {
			switch entry.Action {
			case conflictSkip:
				skipped++
				continue
			case conflictOverwrite:
				if err := v.ctx.RoleEngine.Save(&r); err != nil {
					skipped++
					continue
				}
				overwritten++
			case conflictRename:
				r.Name = entry.NewName
				if err := v.ctx.RoleEngine.Create(&r); err != nil {
					skipped++
					continue
				}
				renamed++
			}
		} else {
			if err := v.ctx.RoleEngine.Create(&r); err != nil {
				skipped++
				continue
			}
			imported++
		}
	}

	if list, err := v.ctx.RoleEngine.List(); err == nil {
		v.items = list
	}

	return func() tea.Msg {
		return statusMsg(buildImportStatus("role", imported, overwritten, renamed, skipped))
	}
}

func (v *rolesViewModel) doImportRoles(toImport []datamodel.Role) tea.Cmd {
	var imported int
	for _, r := range toImport {
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
		if vals[3] != "" {
			for _, c := range strings.Split(vals[3], ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					comps = append(comps, c)
				}
			}
		}

		var tools []string
		if vals[4] != "" {
			for _, t := range strings.Split(vals[4], ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tools = append(tools, t)
				}
			}
		}

		msgMode := vals[5]
		if msgMode == "" {
			msgMode = "interrupt"
		}

		r := &datamodel.Role{
			Name:        vals[0],
			Description: vals[1],
			APIType:     datamodel.APIType(vals[2]),
			Components:  comps,
			Tools:       tools,
			MessageMode: msgMode,
			Model:       vals[6],
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

	updates := &datamodel.Role{
		Description: vals[0],
		APIType:     datamodel.APIType(vals[1]),
		Components:  comps,
		Tools:       tools,
		MessageMode: vals[4],
		Model:       vals[5],
	}
	if err := v.ctx.RoleEngine.Update(name, updates); err != nil {
		return func() tea.Msg { return statusMsg("Update failed: " + err.Error()) }
	}

	if v.cursor < len(v.items) {
		if updates.Description != "" {
			v.items[v.cursor].Description = updates.Description
		}
		if updates.APIType != "" {
			v.items[v.cursor].APIType = updates.APIType
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

	if v.conflictResolver.IsActive() {
		return v.conflictResolver.View()
	}

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

	header := fmt.Sprintf("  %-18s %-14s %-12s %-18s %-20s %-20s", "NAME", "API_TYPE", "MODE", "MODEL", "COMPONENTS", "TOOLS")
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
		apiType := string(r.APIType)
		if apiType == "" {
			apiType = string(datamodel.APITypeChatCompletion)
		}
		row := fmt.Sprintf("  %-18s %-14s %-12s %-18s %-20s %-20s",
			truncate(r.Name, 18), truncate(apiType, 14), truncate(mode, 12), truncate(modelName, 18), comps, tools)
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
		b.WriteString(valueStyle.Render(fmt.Sprintf("  API Type:   %s", r.APIType)))
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
