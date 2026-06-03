package tui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	datamodel "github.com/Lyoomu/TAC/Agent/internal/model"
)

func importExportDirs() (inputDir, outputDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	inputDir = filepath.Join(home, ".tac", "Agent", "Input")
	outputDir = filepath.Join(home, ".tac", "Agent", "Output")
	return
}

func exportModels(items []datamodel.Model, selected []int) error {
	_, outputDir := importExportDirs()
	dir := filepath.Join(outputDir, "Models")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		m := items[idx]
		m.APIKey = ""
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal model %s: %w", m.Name, err)
		}
		path := filepath.Join(dir, m.Name+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write model %s: %w", m.Name, err)
		}
	}
	return nil
}

func exportRoles(items []datamodel.Role, selected []int) error {
	_, outputDir := importExportDirs()
	dir := filepath.Join(outputDir, "Roles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		r := items[idx]
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal role %s: %w", r.Name, err)
		}
		path := filepath.Join(dir, r.Name+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write role %s: %w", r.Name, err)
		}
	}
	return nil
}

func exportComponents(items []datamodel.Component, selected []int) error {
	_, outputDir := importExportDirs()
	dir := filepath.Join(outputDir, "Components")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		c := items[idx]
		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal component %s: %w", c.Name, err)
		}
		path := filepath.Join(dir, c.Name+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write component %s: %w", c.Name, err)
		}
	}
	return nil
}

type ToolExportData struct {
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Version             string          `json:"version"`
	Language            string          `json:"language"`
	Type                string          `json:"type"`
	Parameters          json.RawMessage `json:"parameters"`
	Strict              bool            `json:"strict"`
	Dependencies        []string        `json:"dependencies"`
	RequiresCompilation bool            `json:"requires_compilation"`
	IsBinary            bool            `json:"is_binary"`
	SourceAvailable     bool            `json:"source_available"`
	RuntimeRequirement  string          `json:"runtime_requirement"`
}

func exportTools(items []*datamodel.Tool, selected []int) error {
	_, outputDir := importExportDirs()
	dir := filepath.Join(outputDir, "Tools")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		t := items[idx]
		toolDir := filepath.Join(dir, t.Name)
		scriptsDir := filepath.Join(toolDir, "Scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			return fmt.Errorf("create tool dir %s: %w", t.Name, err)
		}

		exportData := ToolExportData{
			Name:                t.Name,
			Description:         t.Config.Description,
			Version:             t.Version,
			Language:            t.Language,
			Type:                t.Config.Type,
			Parameters:          t.Config.Parameters,
			Strict:              t.Config.Strict,
			Dependencies:        t.Dependencies,
			RequiresCompilation: t.RequiresCompilation,
			IsBinary:            t.IsBinary,
			SourceAvailable:     t.SourceAvailable,
			RuntimeRequirement:  t.RuntimeRequirement,
		}
		data, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal tool %s: %w", t.Name, err)
		}
		if err := os.WriteFile(filepath.Join(toolDir, t.Name+".json"), data, 0644); err != nil {
			return fmt.Errorf("write tool config %s: %w", t.Name, err)
		}

		if t.ScriptDir != "" {
			_ = filepath.WalkDir(t.ScriptDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(t.ScriptDir, path)
				dest := filepath.Join(scriptsDir, rel)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(dest, content, 0644)
			})
		}
	}
	return nil
}

func scanImportModels() ([]datamodel.Model, error) {
	inputDir, _ := importExportDirs()
	dir := filepath.Join(inputDir, "Models")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []datamodel.Model
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var m datamodel.Model
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Name == "" {
			m.Name = strings.TrimSuffix(entry.Name(), ".json")
		}
		result = append(result, m)
	}
	return result, nil
}

func scanImportRoles() ([]datamodel.Role, error) {
	inputDir, _ := importExportDirs()
	dir := filepath.Join(inputDir, "Roles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []datamodel.Role
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var r datamodel.Role
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		if r.Name == "" {
			r.Name = strings.TrimSuffix(entry.Name(), ".json")
		}
		result = append(result, r)
	}
	return result, nil
}

func scanImportComponents() ([]datamodel.Component, error) {
	inputDir, _ := importExportDirs()
	dir := filepath.Join(inputDir, "Components")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []datamodel.Component
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var c datamodel.Component
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if c.Name == "" {
			c.Name = strings.TrimSuffix(entry.Name(), ".json")
		}
		result = append(result, c)
	}
	return result, nil
}

type ImportToolEntry struct {
	Config    ToolExportData
	ToolDir   string
	ScriptDir string
}

func scanImportTools() ([]ImportToolEntry, error) {
	inputDir, _ := importExportDirs()
	dir := filepath.Join(inputDir, "Tools")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []ImportToolEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		toolDir := filepath.Join(dir, entry.Name())

		configPath := filepath.Join(toolDir, entry.Name()+".json")
		data, err := os.ReadFile(configPath)
		if err != nil {

			subEntries, _ := os.ReadDir(toolDir)
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".json") {
					configPath = filepath.Join(toolDir, se.Name())
					data, err = os.ReadFile(configPath)
					if err == nil {
						break
					}
				}
			}
			if err != nil {
				continue
			}
		}
		var cfg ToolExportData
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if cfg.Name == "" {
			cfg.Name = entry.Name()
		}
		scriptsDir := filepath.Join(toolDir, "Scripts")
		if _, err := os.Stat(scriptsDir); err != nil {
			scriptsDir = toolDir
		}
		result = append(result, ImportToolEntry{
			Config:    cfg,
			ToolDir:   toolDir,
			ScriptDir: scriptsDir,
		})
	}
	return result, nil
}
