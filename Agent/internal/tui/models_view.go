package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

type modelsViewModel struct {
	ctx        *AppContext
	width      int
	height     int
	items      []datamodel.Model
	cursor     int
	form       *formModel
	selector   *selectorModel
	detailMode bool

	importItems []datamodel.Model
}

func newModelsViewModel(ctx *AppContext) *modelsViewModel {
	return &modelsViewModel{ctx: ctx, form: newFormModel(), selector: newSelectorModel()}
}

func (v *modelsViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *modelsViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return modelsDataMsg{}
	}
}

type modelsDataMsg struct{}

func (v *modelsViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case modelsDataMsg:
		if v.ctx.ModelsEngine != nil {
			if list, err := v.ctx.ModelsEngine.List(); err == nil {
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
			v.form.startCreate("Create Model", []formField{
				{label: "API Type", required: true, options: []string{
					string(datamodel.APITypeChatCompletion),
					string(datamodel.APITypeResponses),
					string(datamodel.APITypeAnthropic),
				}},
				{label: "Name", placeholder: "unique model name", required: true},
				{label: "Model", placeholder: "model identifier (e.g. gpt-4o)", required: true},
				{label: "Base URL", placeholder: "API base URL (e.g. https://api.openai.com/v1)", required: true},
				{label: "API Key", placeholder: "API key (optional)", required: false},
			})
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				m := v.items[v.cursor]
				apiType := string(m.APIType)
				if apiType == "" {
					apiType = string(datamodel.APITypeChatCompletion)
				}
				v.form.startEdit("Edit Model: "+m.Name, []formField{
					{label: "API Type", value: apiType, required: true, options: []string{
						string(datamodel.APITypeChatCompletion),
						string(datamodel.APITypeResponses),
						string(datamodel.APITypeAnthropic),
					}},
					{label: "Model", placeholder: "model identifier", value: m.Model, required: true},
					{label: "Base URL", placeholder: "API base URL", value: m.BaseURL, required: true},
					{label: "API Key", placeholder: "API key (leave empty to keep current)", value: "", required: false},
				})
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) && v.ctx.ModelsEngine != nil {
				name := v.items[v.cursor].Name
				if err := v.ctx.ModelsEngine.Delete(name); err != nil {
					return func() tea.Msg { return statusMsg("Delete failed: " + err.Error()) }
				}
				if list, err := v.ctx.ModelsEngine.List(); err == nil {
					v.items = list
					if v.cursor >= len(v.items) {
						v.cursor = len(v.items) - 1
						if v.cursor < 0 {
							v.cursor = 0
						}
					}
				}
				return func() tea.Msg { return statusMsg("Deleted: " + name) }
			}
		case "enter":
			if v.cursor < len(v.items) {
				v.detailMode = !v.detailMode
			}
		case "i", "I":
			items, err := scanImportModels()
			if err != nil {
				return func() tea.Msg { return statusMsg("Scan import failed: " + err.Error()) }
			}
			if len(items) == 0 {
				return func() tea.Msg { return statusMsg("No model files found in Input/Models") }
			}
			v.importItems = items
			var names []string
			for _, m := range items {
				names = append(names, m.Name)
			}
			v.selector.start(selectorImport, "Select Models to Import", names)
			return nil
		case "o", "O":
			if len(v.items) == 0 {
				return func() tea.Msg { return statusMsg("No models to export") }
			}
			var names []string
			for _, m := range v.items {
				names = append(names, m.Name)
			}
			v.selector.start(selectorExport, "Select Models to Export", names)
			return nil
		case "esc":
			v.detailMode = false
		}
	}
	return nil
}

func (v *modelsViewModel) handleSelectorDone() tea.Cmd {
	selected := v.selector.SelectedIndices()
	if len(selected) == 0 {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("No items selected") }
	}
	if v.selector.mode == selectorExport {
		if err := exportModels(v.items, selected); err != nil {
			return func() tea.Msg { return statusMsg("Export failed: " + err.Error()) }
		}
		_, outDir := importExportDirs()
		return func() tea.Msg {
			return statusMsg(fmt.Sprintf("Exported %d model(s) to %s", len(selected), filepath.Join(outDir, "Models")))
		}
	}

	if v.ctx.ModelsEngine == nil {
		v.importItems = nil
		return func() tea.Msg { return statusMsg("ModelsEngine not available") }
	}
	var imported int
	for _, idx := range selected {
		if idx < 0 || idx >= len(v.importItems) {
			continue
		}
		m := v.importItems[idx]
		if err := v.ctx.ModelsEngine.Create(&m); err != nil {
			continue
		}
		imported++
	}
	v.importItems = nil
	if list, err := v.ctx.ModelsEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg(fmt.Sprintf("Imported %d model(s)", imported)) }
}

func (v *modelsViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()

	if v.form.mode == formModeCreate {

		if v.ctx.ModelsEngine == nil {
			return func() tea.Msg { return statusMsg("ModelsEngine not available") }
		}
		m := &datamodel.Model{
			APIType: datamodel.APIType(vals[0]),
			Name:    vals[1],
			Model:   vals[2],
			BaseURL: vals[3],
			APIKey:  vals[4],
		}
		if err := v.ctx.ModelsEngine.Create(m); err != nil {
			return func() tea.Msg { return statusMsg("Create failed: " + err.Error()) }
		}
		if list, err := v.ctx.ModelsEngine.List(); err == nil {
			v.items = list
		}
		return func() tea.Msg { return statusMsg("Created model: " + m.Name) }
	}

	if v.ctx.ModelsEngine == nil || v.cursor >= len(v.items) {
		return func() tea.Msg { return statusMsg("ModelsEngine not available") }
	}
	name := v.items[v.cursor].Name
	updates := &datamodel.Model{
		APIType: datamodel.APIType(vals[0]),
		Model:   vals[1],
		BaseURL: vals[2],
		APIKey:  vals[3],
	}
	if err := v.ctx.ModelsEngine.Update(name, updates); err != nil {
		return func() tea.Msg { return statusMsg("Update failed: " + err.Error()) }
	}
	if list, err := v.ctx.ModelsEngine.List(); err == nil {
		v.items = list
	}
	return func() tea.Msg { return statusMsg("Updated model: " + name) }
}

func (v *modelsViewModel) View() string {

	if v.selector.IsActive() {
		return v.selector.View()
	}

	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Models"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No models registered. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-20s %-18s %-25s %-30s", "NAME", "API_TYPE", "MODEL", "BASE_URL")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	for i, m := range v.items {
		apiType := string(m.APIType)
		if apiType == "" {
			apiType = string(datamodel.APITypeChatCompletion)
		}
		row := fmt.Sprintf("  %-20s %-18s %-25s %-30s", truncate(m.Name, 20), truncate(apiType, 18), truncate(m.Model, 25), truncate(m.BaseURL, 30))
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if v.detailMode && v.cursor < len(v.items) {
		m := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Model Details"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Name:    %s", m.Name)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  API:     %s", string(m.APIType))))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Model:   %s", m.Model)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  BaseURL: %s", m.BaseURL)))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Details  [c] Create  [e] Edit  [d] Delete  [i] Import  [o] Export"))
	}

	return b.String()
}
