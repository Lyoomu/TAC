package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrWorkspaceExists   = errors.New("workspace already exists")
	ErrInvalidName       = errors.New("invalid workspace name")
	ErrInvalidPath       = errors.New("invalid workspace path")
	ErrPathAlreadyBound  = errors.New("path already bound to another workspace")
)

const tacDirName = ".tac"

type Engine struct {
	workspaces map[string]*model.Workspace
	configPath string
	mu         sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		workspaces: make(map[string]*model.Workspace),
		configPath: defaultConfigPath(),
	}
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tac", "workspaces.yaml")
}

func (e *Engine) Load() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := os.Stat(e.configPath); os.IsNotExist(err) {

		return nil
	}

	data, err := os.ReadFile(e.configPath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var cfg struct {
		Workspaces []model.Workspace `yaml:"workspaces"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	for _, ws := range cfg.Workspaces {
		w := ws
		e.workspaces[w.Name] = &w
	}
	return nil
}

func (e *Engine) Save() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var cfg struct {
		Workspaces []model.Workspace `yaml:"workspaces"`
	}
	for _, ws := range e.workspaces {
		cfg.Workspaces = append(cfg.Workspaces, *ws)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(e.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func (e *Engine) Bind(name, path string) error {
	if name == "" {
		return ErrInvalidName
	}
	if path == "" {
		return ErrInvalidPath
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workspaces[name]; exists {
		return ErrWorkspaceExists
	}

	for _, ws := range e.workspaces {
		if ws.Path == absPath {
			return ErrPathAlreadyBound
		}
	}

	tacPath := filepath.Join(absPath, tacDirName)
	if err := os.MkdirAll(tacPath, 0755); err != nil {
		return fmt.Errorf("create .tac directory: %w", err)
	}

	now := time.Now()
	e.workspaces[name] = &model.Workspace{
		Name:      name,
		Path:      absPath,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return e.saveUnsafe()
}

func (e *Engine) Unbind(name string) error {
	if name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	ws, exists := e.workspaces[name]
	if !exists {
		return ErrWorkspaceNotFound
	}

	delete(e.workspaces, name)

	_ = ws

	return e.saveUnsafe()
}

func (e *Engine) List() []model.Workspace {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]model.Workspace, 0, len(e.workspaces))
	for _, ws := range e.workspaces {
		list = append(list, *ws)
	}
	return list
}

func (e *Engine) Get(name string) (*model.Workspace, error) {
	if name == "" {
		return nil, ErrInvalidName
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	ws, exists := e.workspaces[name]
	if !exists {
		return nil, ErrWorkspaceNotFound
	}
	clone := *ws
	return &clone, nil
}

func (e *Engine) Exists(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.workspaces[name]
	return exists
}

func (e *Engine) Activate(name string) error {
	if name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workspaces[name]; !exists {
		return ErrWorkspaceNotFound
	}

	for _, ws := range e.workspaces {
		ws.IsActive = false
	}

	e.workspaces[name].IsActive = true

	return e.saveUnsafe()
}

func (e *Engine) GetActive() (*model.Workspace, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, ws := range e.workspaces {
		if ws.IsActive {
			clone := *ws
			return &clone, nil
		}
	}
	return nil, ErrWorkspaceNotFound
}

func (e *Engine) GetTACPath(name string) (string, error) {
	ws, err := e.Get(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(ws.Path, tacDirName), nil
}

func (e *Engine) GetActiveTACPath() (string, error) {
	ws, err := e.GetActive()
	if err != nil {
		return "", err
	}
	return filepath.Join(ws.Path, tacDirName), nil
}

func (e *Engine) saveUnsafe() error {
	var cfg struct {
		Workspaces []model.Workspace `yaml:"workspaces"`
	}
	for _, ws := range e.workspaces {
		cfg.Workspaces = append(cfg.Workspaces, *ws)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(e.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}
