package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

func MainFileForScriptType(scriptType string) string {
	switch strings.ToLower(scriptType) {
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

func SaveFiles(serverName, toolName string, files map[string][]byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	toolDir := filepath.Join(home, ".tac", "tools", sanitizeFilename(serverName), sanitizeFilename(toolName))
	if err := os.RemoveAll(toolDir); err != nil {
		return "", fmt.Errorf("clear tool dir: %w", err)
	}
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return "", fmt.Errorf("create tool dir: %w", err)
	}
	for name, data := range files {
		rel, err := safeRelativePath(name)
		if err != nil {
			return "", err
		}
		filePath := filepath.Join(toolDir, rel)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", fmt.Errorf("create subdir: %w", err)
		}
		mode := os.FileMode(0644)
		if filepath.Base(rel) == "main" || filepath.Ext(rel) == ".exe" {
			mode = 0755
		}
		if err := os.WriteFile(filePath, data, mode); err != nil {
			return "", fmt.Errorf("write file %s: %w", name, err)
		}
	}
	return toolDir, nil
}

func Execute(info model.ToolInfo, args string) (string, error) {
	if info.LocalPath == "" {
		return "", fmt.Errorf("tool %s is not downloaded", info.Name)
	}
	mainFile := MainFileForScriptType(info.Language)
	if mainFile == "" {
		return "", fmt.Errorf("unsupported tool script type: %s", info.Language)
	}
	mainPath := filepath.Join(info.LocalPath, mainFile)
	if _, err := os.Stat(mainPath); err != nil {
		return "", fmt.Errorf("tool main file not found: %s", mainPath)
	}

	cmdName := mainPath
	cmdArgs := []string{args}
	switch strings.ToLower(info.Language) {
	case "python":
		cmdName = "python"
		cmdArgs = []string{mainPath, args}
	case "javascripts":
		cmdName = "node"
		cmdArgs = []string{mainPath, args}
	case "typescripts":
		cmdName = "ts-node"
		cmdArgs = []string{mainPath, args}
	case "linux":
		if runtime.GOOS == "windows" {
			return "", fmt.Errorf("linux tool cannot be executed on windows")
		}
	case "win":
		if runtime.GOOS != "windows" {
			return "", fmt.Errorf("windows tool cannot be executed on %s", runtime.GOOS)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = info.LocalPath
	cmd.Stdin = strings.NewReader(args)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errorJSON(fmt.Sprintf("execute tool %s: %v: %s", info.Name, err, strings.TrimSpace(stderr.String()))), nil
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = `{"status":"ok"}`
	}
	return out, nil
}

func FindLoadedTool(roles []model.LoadedRole, serverName, roleName, toolName string) (model.ToolInfo, bool) {
	for _, role := range roles {
		if role.ServerName != serverName || role.RoleName != roleName {
			continue
		}
		for _, tool := range role.Tools {
			if tool.Name == toolName {
				return tool, true
			}
		}
	}
	return model.ToolInfo{}, false
}

func errorJSON(message string) string {
	data, _ := json.Marshal(map[string]string{"error": message})
	return string(data)
}

func safeRelativePath(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("invalid tool file path: %s", name)
	}
	return clean, nil
}

func sanitizeFilename(name string) string {
	result := []rune(name)
	for i, r := range result {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result[i] = '_'
		}
	}
	return string(result)
}
