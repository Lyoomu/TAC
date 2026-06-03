package tool

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var ErrToolNotFound = errors.New("tool not found")

type Engine struct {
	tools    map[string]*model.Tool
	mu       sync.RWMutex
	toolPath string
	db       *sql.DB
}

func NewEngine(dbConn *sql.DB, toolPath string) *Engine {
	return &Engine{
		tools:    make(map[string]*model.Tool),
		toolPath: toolPath,
		db:       dbConn,
	}
}

func (e *Engine) Register() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return errors.New("tool engine database is nil")
	}
	if err := os.MkdirAll(e.toolPath, 0755); err != nil {
		return fmt.Errorf("create tool path %s: %w", e.toolPath, err)
	}
	if err := e.ensureBuiltInTools(); err != nil {
		return err
	}
	return e.loadFromDB()
}

func (e *Engine) ensureBuiltInTools() error {
	scriptDir := filepath.Join(e.toolPath, "get_current_time")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return fmt.Errorf("create builtin tool dir: %w", err)
	}
	mainPath := filepath.Join(scriptDir, mainFileForScriptType("python"))
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		content := `import json
from datetime import datetime, timezone

if __name__ == "__main__":
    print(json.dumps({"time": datetime.now(timezone.utc).isoformat()}))
`
		if err := os.WriteFile(mainPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write builtin tool main: %w", err)
		}
	}

	parameters := `{"type":"object","properties":{},"required":[]}`
	_, err := e.db.Exec(`INSERT INTO tools (
name, description, type, parameters, strict, version, script_type, script_dir,
dependencies, requires_compilation, is_binary, source_available, runtime_requirement
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name) DO NOTHING`,
		"get_current_time", "获取当前的系统时间", "function", parameters, 0, "1.0.0", "python", scriptDir,
		"[]", 0, 0, 1, "python")
	if err != nil {
		return fmt.Errorf("seed builtin tool get_current_time: %w", err)
	}
	return nil
}

func (e *Engine) loadFromDB() error {
	rows, err := e.db.Query(`SELECT name, description, type, parameters, strict, version, script_type, script_dir,
dependencies, requires_compilation, is_binary, source_available, runtime_requirement FROM tools ORDER BY name`)
	if err != nil {
		return fmt.Errorf("query tools: %w", err)
	}
	defer rows.Close()

	loaded := make(map[string]*model.Tool)
	for rows.Next() {
		var name, toolType, parameters, scriptType string
		var description, version, scriptDir, dependenciesJSON, runtimeRequirement sql.NullString
		var strict, requiresCompilation, isBinary, sourceAvailable int
		if err := rows.Scan(&name, &description, &toolType, &parameters, &strict, &version, &scriptType, &scriptDir, &dependenciesJSON, &requiresCompilation, &isBinary, &sourceAvailable, &runtimeRequirement); err != nil {
			return fmt.Errorf("scan tool: %w", err)
		}
		scriptDirValue := nullableString(scriptDir)
		if scriptDirValue == "" {
			scriptDirValue = filepath.Join(e.toolPath, name)
		}
		if !filepath.IsAbs(scriptDirValue) {
			scriptDirValue = filepath.Join(e.toolPath, scriptDirValue)
		}

		var dependencies []string
		_ = json.Unmarshal([]byte(defaultString(nullableString(dependenciesJSON), "[]")), &dependencies)
		cfg := model.ToolConfig{
			Type:        defaultString(toolType, "function"),
			Name:        name,
			Description: nullableString(description),
			Parameters:  json.RawMessage(defaultString(parameters, `{"type":"object","properties":{},"required":[]}`)),
			Strict:      strict != 0,
		}
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal tool config %s: %w", name, err)
		}
		loaded[name] = &model.Tool{
			Name:                name,
			Version:             nullableString(version),
			Config:              cfg,
			ConfigJSON:          configJSON,
			ScriptDir:           scriptDirValue,
			Scripts:             listFiles(scriptDirValue),
			MainFile:            mainFileForScriptType(scriptType),
			Language:            scriptType,
			Dependencies:        dependencies,
			RequiresCompilation: requiresCompilation != 0,
			IsBinary:            isBinary != 0,
			SourceAvailable:     sourceAvailable != 0,
			RuntimeRequirement:  nullableString(runtimeRequirement),
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tools: %w", err)
	}
	e.tools = loaded
	return nil
}

func (e *Engine) AddTool(name, description, toolType, parameters string, strict bool,
	version, scriptType, scriptDir, dependencies string,
	requiresCompilation, isBinary, sourceAvailable bool, runtimeRequirement string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := e.db.Exec(`INSERT INTO tools (
name, description, type, parameters, strict, version, script_type, script_dir,
dependencies, requires_compilation, is_binary, source_available, runtime_requirement
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
description=excluded.description,
type=excluded.type,
parameters=excluded.parameters,
strict=excluded.strict,
version=excluded.version,
script_type=excluded.script_type,
script_dir=excluded.script_dir,
dependencies=excluded.dependencies,
requires_compilation=excluded.requires_compilation,
is_binary=excluded.is_binary,
source_available=excluded.source_available,
runtime_requirement=excluded.runtime_requirement`,
		name, description, defaultString(toolType, "function"), parameters, boolToInt(strict), version,
		normalizeScriptType(scriptType), scriptDir, dependencies,
		boolToInt(requiresCompilation), boolToInt(isBinary), boolToInt(sourceAvailable), runtimeRequirement)
	if err != nil {
		return fmt.Errorf("add tool %s: %w", name, err)
	}
	return nil
}

func (e *Engine) List() []*model.Tool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]*model.Tool, 0, len(e.tools))
	for _, t := range e.tools {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

func (e *Engine) GetInfo(name string) (json.RawMessage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, ok := e.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}
	return t.ConfigJSON, nil
}

func (e *Engine) Pull(name string) (map[string][]byte, error) {
	return e.GetToolFiles(name)
}

func (e *Engine) GetDetail(name string) (*model.Tool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, ok := e.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}
	clone := *t
	return &clone, nil
}

func (e *Engine) GetToolFiles(name string) (map[string][]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, ok := e.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}
	return readDirFiles(t.ScriptDir)
}

func (e *Engine) Delete(name string) error {
	if name == "" {
		return errors.New("tool name is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.tools[name]; !ok {
		return ErrToolNotFound
	}
	if _, err := e.db.Exec("DELETE FROM tools WHERE name = ?", name); err != nil {
		return fmt.Errorf("delete tool %s: %w", name, err)
	}
	delete(e.tools, name)
	return nil
}

func (e *Engine) ValidateTools(clientTools map[string]string) (missing []string, outdated []string, err error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for name, tool := range e.tools {
		clientVersion, ok := clientTools[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if clientVersion != tool.Version {
			outdated = append(outdated, name)
		}
	}
	return
}

func readDirFiles(root string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	if root == "" {
		return result, nil
	}
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read script %s: %w", rel, err)
		}
		result[filepath.ToSlash(rel)] = data
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func listFiles(root string) []string {
	filesMap, err := readDirFiles(root)
	if err != nil {
		return nil
	}
	files := make([]string, 0, len(filesMap))
	for name := range filesMap {
		files = append(files, name)
	}
	sort.Strings(files)
	return files
}

func mainFileForScriptType(scriptType string) string {
	switch scriptType {
	case "win":
		return "main.exe"
	case "linux":
		return "main"
	case "python":
		return "main.py"
	case "javascripts":
		return "main.js"
	case "typescripts":
		return "main.ts"
	default:
		return ""
	}
}

func normalizeScriptType(language string) string {
	switch strings.ToLower(language) {
	case "win", "windows", "exe":
		return "win"
	case "linux":
		return "linux"
	case "python", "py":
		return "python"
	case "javascript", "javascripts", "js":
		return "javascripts"
	case "typescript", "typescripts", "ts":
		return "typescripts"
	default:
		return ""
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
