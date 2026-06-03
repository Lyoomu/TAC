package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

type componentsViewModel struct {
	ctx        *AppContext
	width      int
	height     int
	items      []datamodel.Component
	cursor     int
	form       *formModel
	selector   *selectorModel
	detailMode bool

	importItems []datamodel.Component
}

func newComponentsViewModel(ctx *AppContext) *componentsViewModel {
	return &componentsViewModel{ctx: ctx, form: newFormModel(), selector: newSelectorModel()}
}

func (v *componentsViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *componentsViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return componentsDataMsg{}
	}
}

type componentsDataMsg struct{}

func (v *componentsViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case componentsDataMsg:
		if v.ctx.ComponentEngine != nil {
			if list, err := v.ctx.ComponentEngine.List(); err == nil {
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
		case "c", "C":
			v.form.startCreate("Create Component", []formField{
				{label: "Name", placeholder: "unique component name", required: true},
				{label: "Type", required: true, options: []string{
					string(datamodel.ComponentStatic),
					string(datamodel.ComponentEmbedded),
				}},
				{label: "Description", placeholder: "component description", required: false},
				{label: "Content", placeholder: "prompt content text", required: true},
			})
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				c := v.items[v.cursor]
				v.form.startEdit("Edit Component: "+c.Name, []formField{
					{label: "Type", value: string(c.Type), required: true, options: []string{
						string(datamodel.ComponentStatic),
						string(datamodel.ComponentEmbedded),
					}},
					{label: "Description", placeholder: "component description", value: c.Description, required: false},
					{label: "Content", placeholder: "prompt content text", value: c.Content, required: true},
				})
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) && v.ctx.ComponentEngine != nil {
				name := v.items[v.cursor].Name
				if err := v.ctx.ComponentEngine.Delete(name); err != nil {
					return func() tea.Msg { return statusMsg("Delete failed: " + err.Error()) }
				}
				if list, err := v.ctx.ComponentEngine.List(); err == nil {
					v.items = list
					if v.cursor >= len(v.items) {
						v.cursor = len(v.items) - 1
						if v.cursor < 0 {
							v.cursor = 0
						}
					}
				}
				return func() tea.Msg { return statusMsg("Deleted component: " + name) }
			}
		case "enter":
			if v.cursor < len(v.items) {
				v.detailMode = !v.detailMode
			}
		case "i", "I":
			items, err := scanImportComponents()
			if err != nil {
				return func() tea.Msg { return statusMsg("Scan import failed: " + err.Error()) }
			}
			if len(items) == 0 {
				return func() tea.Msg { return statusMsg("No component files found in Input/Components") }
			}
			v.importItems = items
			var names []string
			for _, c := range items {
				names = append(names, c.Name)
			}
			v.selector.start(selectorImport, "Select Components to Import", names)
			return nil
		case "o", "O":
			if len(v.items) == 0 {
				return func() tea.Msg { return statusMsg("No components to export") }
			}
			var names []string
			for _, c := range v.items {
				names = append(names, c.Name)
			}
			v.selector.start(selectorExport, "Select Components to Export", names)
			return nil
		case "esc":
			v.detailMode = false
		}
	}
	return nil
}

func (v *componentsViewModel) handleSelectorDone() tea.Cmd {
	selected := v.selector.SelectedIndices()
	if len(selected) == 0 {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("No items selected") }
	}
	if v.selector.mode == selectorExport {
		if err := exportComponents(v.items, selected); err != nil {
			return func() tea.Msg { return statusMsg("Export failed: " + err.Error()) }
		}
		_, outDir := importExportDirs()
		return func() tea.Msg {
			return statusMsg(fmt.Sprintf("Exported %d component(s) to %s", len(selected), filepath.Join(outDir, "Components")))
		}
	}

	if v.ctx.ComponentEngine == nil {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("ComponentEngine not available") }
	}
	var imported int
	for _, idx := range selected {
		if idx < 0 || idx >= len(v.importItems) {
			continue
		}
		c := v.importItems[idx]
		if err := v.ctx.ComponentEngine.Create(&c); err != nil {
			continue
		}
		imported++
	}
	v.importItems = nil
	if list, err := v.ctx.ComponentEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg(fmt.Sprintf("Imported %d component(s)", imported)) }
}

func (v *componentsViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()

	if v.form.mode == formModeCreate {

		if v.ctx.ComponentEngine == nil {
			return func() tea.Msg { return statusMsg("ComponentEngine not available") }
		}
		c := &datamodel.Component{
			Name:        vals[0],
			Type:        datamodel.ComponentType(vals[1]),
			Description: vals[2],
			Content:     vals[3],
		}
		if err := v.ctx.ComponentEngine.Create(c); err != nil {
			return func() tea.Msg { return statusMsg("Create failed: " + err.Error()) }
		}
		if list, err := v.ctx.ComponentEngine.List(); err == nil {
			v.items = list
		}
		return func() tea.Msg { return statusMsg("Created component: " + c.Name) }
	}

	if v.ctx.ComponentEngine == nil || v.cursor >= len(v.items) {
		return func() tea.Msg { return statusMsg("ComponentEngine not available") }
	}
	name := v.items[v.cursor].Name
	updates := &datamodel.Component{
		Type:        datamodel.ComponentType(vals[0]),
		Description: vals[1],
		Content:     vals[2],
	}
	if err := v.ctx.ComponentEngine.Update(name, updates); err != nil {
		return func() tea.Msg { return statusMsg("Update failed: " + err.Error()) }
	}
	if list, err := v.ctx.ComponentEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg("Updated component: " + name) }
}

func (v *componentsViewModel) View() string {

	if v.selector.IsActive() {
		return v.selector.View()
	}

	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Components"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No components defined. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-20s %-12s %-40s", "NAME", "TYPE", "DESCRIPTION")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	for i, c := range v.items {
		row := fmt.Sprintf("  %-20s %-12s %-40s",
			truncate(c.Name, 20), truncate(string(c.Type), 12), truncate(c.Description, 40))
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if v.detailMode && v.cursor < len(v.items) {
		c := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Component Details"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Name:    %s", c.Name)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Type:    %s", string(c.Type))))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Desc:    %s", c.Description)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render("  Content:"))
		b.WriteString("\n")
		contentWidth := v.width - 6
		if contentWidth < 20 {
			contentWidth = 20
		}
		wrapped := wrapText(c.Content, contentWidth)
		for _, line := range strings.Split(wrapped, "\n") {
			b.WriteString(valueStyle.Render("    " + line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Details  [c] Create  [e] Edit  [d] Delete  [i] Import  [o] Export"))
	}

	return b.String()
}
