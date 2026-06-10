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
	ctx              *AppContext
	width            int
	height           int
	items            []*datamodel.Tool
	cursor           int
	selector         *selectorModel
	conflictResolver *conflictResolverModel
	detailMode       bool

	importToolEntries []ImportToolEntry
	pendingImport     []ImportToolEntry
}

func newToolsViewModel(ctx *AppContext) *toolsViewModel {
	return &toolsViewModel{
		ctx:              ctx,
		selector:         newSelectorModel(),
		conflictResolver: newConflictResolverModel(),
	}
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

		if v.conflictResolver.IsActive() {
			done, cancelled := v.conflictResolver.Update(msg)
			if cancelled {
				v.pendingImport = nil
				return nil
			}
			if done {
				return v.executeToolImport()
			}
			return nil
		}

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
				return func() tea.Msg { return statusMsg("No tool folders found in Import/Tools") }
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

	var toImport []ImportToolEntry
	for _, idx := range selected {
		if idx >= 0 && idx < len(v.importToolEntries) {
			toImport = append(toImport, v.importToolEntries[idx])
		}
	}

	conflicts := v.detectToolConflicts(toImport)
	if len(conflicts) > 0 {
		v.pendingImport = toImport
		existingNames := v.buildToolNameSet()
		v.conflictResolver.start("Tool Import Conflicts", conflicts, existingNames)
		return nil
	}

	return v.doImportTools(toImport)
}

func (v *toolsViewModel) detectToolConflicts(toImport []ImportToolEntry) []*conflictEntry {
	existing := make(map[string]*datamodel.Tool)
	for _, t := range v.items {
		existing[t.Name] = t
	}

	var conflicts []*conflictEntry
	for _, entry := range toImport {
		if t, ok := existing[entry.Config.Name]; ok {
			info := fmt.Sprintf("version: %s", t.Version)
			conflicts = append(conflicts, &conflictEntry{
				Name:         entry.Config.Name,
				ResourceType: "tool",
				ExistingInfo: info,
			})
		}
	}
	return conflicts
}

func (v *toolsViewModel) buildToolNameSet() map[string]bool {
	names := make(map[string]bool)
	for _, t := range v.items {
		names[t.Name] = true
	}
	return names
}

func (v *toolsViewModel) executeToolImport() tea.Cmd {
	resolved := v.conflictResolver.GetResolved()
	v.conflictResolver.cancel()

	toImport := v.pendingImport
	v.pendingImport = nil
	v.importToolEntries = nil

	var imported, overwritten, renamed, skipped int
	for _, entry := range toImport {
		name := entry.Config.Name
		if resolvedEntry, ok := resolved[name]; ok {
			switch resolvedEntry.Action {
			case conflictSkip:
				skipped++
				continue
			case conflictOverwrite:
				if err := v.importTool(entry); err != nil {
					skipped++
					continue
				}
				overwritten++
			case conflictRename:
				entry.Config.Name = resolvedEntry.NewName
				if err := v.importTool(entry); err != nil {
					skipped++
					continue
				}
				renamed++
			}
		} else {
			if err := v.importTool(entry); err != nil {
				skipped++
				continue
			}
			imported++
		}
	}

	_ = v.ctx.ToolEngine.Register()
	v.items = v.ctx.ToolEngine.List()
	return func() tea.Msg {
		return statusMsg(buildImportStatus("tool", imported, overwritten, renamed, skipped))
	}
}

func (v *toolsViewModel) doImportTools(toImport []ImportToolEntry) tea.Cmd {
	var imported int
	for _, entry := range toImport {
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

	if v.conflictResolver.IsActive() {
		return v.conflictResolver.View()
	}

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
