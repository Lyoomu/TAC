package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

type toolsViewModel struct {
	ctx        *AppContext
	width      int
	height     int
	items      []*datamodel.Tool
	cursor     int
	selector   *selectorModel
	detailMode bool

	importToolEntries []ImportToolEntry
}

func newToolsViewModel(ctx *AppContext) *toolsViewModel {
	return &toolsViewModel{ctx: ctx, selector: newSelectorModel()}
}

func (v *toolsViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *toolsViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return toolsDataMsg{}
	}
}

type toolsDataMsg struct{}

func (v *toolsViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case toolsDataMsg:
		if v.ctx.ToolEngine != nil {
			v.items = v.ctx.ToolEngine.List()
		}
	case tea.KeyMsg:

		if v.selector.IsActive() {
			done, cancelled := v.selector.Update(msg)
			if cancelled {
				v.importToolEntries = nil
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
			v.detailMode = false
		case "down", "j":
			if v.cursor < len(v.items)-1 {
				v.cursor++
			}
			v.detailMode = false
		case "r":

			if v.ctx.ToolEngine != nil {
				_ = v.ctx.ToolEngine.Register()
				v.items = v.ctx.ToolEngine.List()
			}
			v.detailMode = false
		case "enter":
			if v.cursor < len(v.items) {
				v.detailMode = !v.detailMode
			}
		case "i", "I":
			entries, err := scanImportTools()
			if err != nil {
				return func() tea.Msg { return statusMsg("Scan import failed: " + err.Error()) }
			}
			if len(entries) == 0 {
				return func() tea.Msg { return statusMsg("No tool folders found in Input/Tools") }
			}
			v.importToolEntries = entries
			var names []string
			for _, e := range entries {
				names = append(names, e.Config.Name)
			}
			v.selector.start(selectorImport, "Select Tools to Import", names)
			return nil
		case "o", "O":
			if len(v.items) == 0 {
				return func() tea.Msg { return statusMsg("No tools to export") }
			}
			var names []string
			for _, t := range v.items {
				names = append(names, t.Name)
			}
			v.selector.start(selectorExport, "Select Tools to Export", names)
			return nil
		case "esc":
			v.detailMode = false
		}
	}
	return nil
}

func (v *toolsViewModel) handleSelectorDone() tea.Cmd {
	selected := v.selector.SelectedIndices()
	if len(selected) == 0 {
		v.importToolEntries = nil
		return func() tea.Msg { return statusMsg("No items selected") }
	}
	if v.selector.mode == selectorExport {
		if err := exportTools(v.items, selected); err != nil {
			return func() tea.Msg { return statusMsg("Export failed: " + err.Error()) }
		}
		_, outDir := importExportDirs()
		return func() tea.Msg {
			return statusMsg(fmt.Sprintf("Exported %d tool(s) to %s", len(selected), filepath.Join(outDir, "Tools")))
		}
	}

	if v.ctx.ToolEngine == nil {
		v.importToolEntries = nil
		return func() tea.Msg { return statusMsg("ToolEngine not available") }
	}
	var imported int
	for _, idx := range selected {
		if idx < 0 || idx >= len(v.importToolEntries) {
			continue
		}
		entry := v.importToolEntries[idx]
		if err := v.importTool(entry); err != nil {
			continue
		}
		imported++
	}
	v.importToolEntries = nil

	_ = v.ctx.ToolEngine.Register()
	v.items = v.ctx.ToolEngine.List()
	return func() tea.Msg { return statusMsg(fmt.Sprintf("Imported %d tool(s)", imported)) }
}

func (v *toolsViewModel) importTool(entry ImportToolEntry) error {
	cfg := entry.Config

	home, _ := os.UserHomeDir()
	toolDir := filepath.Join(home, ".tac", "agent", "tools", cfg.Name)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return err
	}

	if entry.ScriptDir != "" {
		entries, _ := os.ReadDir(entry.ScriptDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(entry.ScriptDir, e.Name()))
			if err != nil {
				continue
			}
			_ = os.WriteFile(filepath.Join(toolDir, e.Name()), data, 0644)
		}
	}

	if v.ctx.ToolEngine == nil {
		return fmt.Errorf("ToolEngine not available")
	}
	dependenciesJSON, _ := json.Marshal(cfg.Dependencies)
	return v.ctx.ToolEngine.AddTool(
		cfg.Name,
		cfg.Description,
		cfg.Type,
		string(cfg.Parameters),
		cfg.Strict,
		cfg.Version,
		cfg.Language,
		toolDir,
		string(dependenciesJSON),
		cfg.RequiresCompilation,
		cfg.IsBinary,
		cfg.SourceAvailable,
		cfg.RuntimeRequirement,
	)
}

func (v *toolsViewModel) View() string {

	if v.selector.IsActive() {
		return v.selector.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Tools"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No tools registered. Use 'tool register' to scan filesystem."))
		return b.String()
	}

	header := fmt.Sprintf("  %-20s %-10s %-35s %-8s", "NAME", "VERSION", "DESCRIPTION", "SCRIPTS")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	for i, t := range v.items {
		desc := t.Config.Description
		row := fmt.Sprintf("  %-20s %-10s %-35s %-8d",
			truncate(t.Name, 20), truncate(t.Version, 10), truncate(desc, 35), len(t.Scripts))
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if v.detailMode && v.cursor < len(v.items) {
		t := v.items[v.cursor]
		b.WriteString(subtitleStyle.Render("  Details: " + t.Name))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Version:  %s", t.Version)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Desc:     %s", t.Config.Description)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Main:     %s", t.MainFile)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  ScriptDir: %s", t.ScriptDir)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Scripts:  %d", len(t.Scripts))))
		if len(t.Scripts) > 0 {
			for _, s := range t.Scripts {
				b.WriteString("\n")
				b.WriteString(valueStyle.Render(fmt.Sprintf("    - %s", s)))
			}
		}
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [Enter/Esc] Close details"))
	} else {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [r] Refresh  [Enter] Details  [i] Import  [o] Export"))
	}

	return b.String()
}
