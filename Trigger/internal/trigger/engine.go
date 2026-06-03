package trigger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

var (
	ErrTriggerNotFound = errors.New("trigger not found")
	ErrTriggerExists   = errors.New("trigger already exists")
	ErrEventNotFound   = errors.New("event not found")
	ErrEventExists     = errors.New("event already exists")
	ErrInvalidType     = errors.New("invalid trigger type")
	ErrEnvPresetNotFound = errors.New("env preset not found")
	ErrEnvPresetExists   = errors.New("env preset already exists")
	ErrInvalidEnvPreset  = errors.New("invalid env preset name")
)

type Engine struct {
	triggers  map[string]*model.Trigger // key = trigger ID
	events    map[string]*model.Event   // key = event ID
	envPresets map[string]*model.EnvPreset // key = preset name
	workspace string                    // 当前 workspace 名称
	basePath  string                    // workspace 磁盘路径
	configDir string                    // .tac 配置目录
	mu        sync.RWMutex

	running map[string]Runner // 正在运行的触发器
	runMu   sync.RWMutex
}

type Runner interface {
	Start(events []*model.Event, executor *Executor) error
	Stop() error
	IsRunning() bool
}

func NewEngine(workspaceName, workspacePath string) *Engine {
	configDir := filepath.Join(workspacePath, ".tac")
	return &Engine{
		triggers:   make(map[string]*model.Trigger),
		events:     make(map[string]*model.Event),
		envPresets: make(map[string]*model.EnvPreset),
		workspace:  workspaceName,
		basePath:   workspacePath,
		configDir:  configDir,
		running:    make(map[string]Runner),
	}
}

func (e *Engine) Load() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	triggerDir := filepath.Join(e.configDir, "triggers")
	if entries, err := os.ReadDir(triggerDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(triggerDir, entry.Name()))
			if err != nil {
				continue
			}
			var t model.Trigger
			if err := yaml.Unmarshal(data, &t); err != nil {
				continue
			}
			e.triggers[t.ID] = &t
		}
	}

	eventDir := filepath.Join(e.configDir, "events")
	if entries, err := os.ReadDir(eventDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(eventDir, entry.Name()))
			if err != nil {
				continue
			}
			var ev model.Event
			if err := yaml.Unmarshal(data, &ev); err != nil {
				continue
			}
			e.events[ev.ID] = &ev
		}
	}

	presetDir := filepath.Join(e.configDir, "env_presets")
	if entries, err := os.ReadDir(presetDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(presetDir, entry.Name()))
			if err != nil {
				continue
			}
			var p model.EnvPreset
			if err := yaml.Unmarshal(data, &p); err != nil {
				continue
			}
			if p.Name == "" {
				p.Name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			}
			e.envPresets[p.Name] = &p
		}
	}

	return nil
}

func (e *Engine) saveTrigger(t *model.Trigger) error {
	triggerDir := filepath.Join(e.configDir, "triggers")
	if err := os.MkdirAll(triggerDir, 0755); err != nil {
		return fmt.Errorf("create trigger dir: %w", err)
	}
	path := filepath.Join(triggerDir, t.ID+".yaml")
	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal trigger: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Engine) saveEvent(ev *model.Event) error {
	eventDir := filepath.Join(e.configDir, "events")
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		return fmt.Errorf("create event dir: %w", err)
	}
	path := filepath.Join(eventDir, ev.ID+".yaml")
	data, err := yaml.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Engine) deleteTriggerFile(id string) error {
	path := filepath.Join(e.configDir, "triggers", id+".yaml")
	return os.Remove(path)
}

func (e *Engine) deleteEventFile(id string) error {
	path := filepath.Join(e.configDir, "events", id+".yaml")
	return os.Remove(path)
}

func (e *Engine) saveEnvPreset(p *model.EnvPreset) error {
	presetDir := filepath.Join(e.configDir, "env_presets")
	if err := os.MkdirAll(presetDir, 0755); err != nil {
		return fmt.Errorf("create env_presets dir: %w", err)
	}
	path := filepath.Join(presetDir, p.Name+".yaml")
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal env preset: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Engine) deleteEnvPresetFile(name string) error {
	path := filepath.Join(e.configDir, "env_presets", name+".yaml")
	return os.Remove(path)
}

func (e *Engine) CreateTrigger(t *model.Trigger) error {
	if t == nil {
		return errors.New("trigger is nil")
	}
	if t.ID == "" {
		t.ID = e.nextTriggerID()
	}
	if t.Type != model.TriggerTypeDirect && t.Type != model.TriggerTypePeriodic && t.Type != model.TriggerTypeEdit {
		return ErrInvalidType
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.triggers[t.ID]; exists {
		return ErrTriggerExists
	}

	t.Workspace = e.workspace
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.EventIDs == nil {
		t.EventIDs = []string{}
	}

	e.triggers[t.ID] = t
	return e.saveTrigger(t)
}

func (e *Engine) nextTriggerID() string {
	for {
		id := fmt.Sprintf("trigger-%d", time.Now().UnixNano())
		if _, exists := e.triggers[id]; !exists {
			return id
		}
	}
}

func (e *Engine) GetTrigger(id string) (*model.Trigger, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, exists := e.triggers[id]
	if !exists {
		return nil, ErrTriggerNotFound
	}
	clone := *t
	return &clone, nil
}

func (e *Engine) UpdateTrigger(id string, updates func(*model.Trigger)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t, exists := e.triggers[id]
	if !exists {
		return ErrTriggerNotFound
	}

	if e.isRunningUnsafe(id) {
		if runner := e.running[id]; runner != nil {
			runner.Stop()
		}
		delete(e.running, id)
	}

	updates(t)
	t.UpdatedAt = time.Now()
	return e.saveTrigger(t)
}

func (e *Engine) DeleteTrigger(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.triggers[id]; !exists {
		return ErrTriggerNotFound
	}

	if e.isRunningUnsafe(id) {
		if runner := e.running[id]; runner != nil {
			runner.Stop()
		}
		delete(e.running, id)
	}

	delete(e.triggers, id)
	return e.deleteTriggerFile(id)
}

func (e *Engine) ListTriggers() []model.Trigger {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]model.Trigger, 0, len(e.triggers))
	for _, t := range e.triggers {
		list = append(list, *t)
	}
	return list
}

func (e *Engine) nextEventID() string {
	for {
		id := fmt.Sprintf("event-%d", time.Now().UnixNano())
		if _, exists := e.events[id]; !exists {
			return id
		}
	}
}

func (e *Engine) CreateEvent(ev *model.Event) error {
	if ev == nil {
		return errors.New("event is nil")
	}
	if ev.ID == "" {
		ev.ID = e.nextEventID()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.events[ev.ID]; exists {
		return ErrEventExists
	}

	ev.Workspace = e.workspace
	now := time.Now()
	ev.CreatedAt = now
	ev.UpdatedAt = now

	e.events[ev.ID] = ev
	return e.saveEvent(ev)
}

func (e *Engine) CreateEnvPreset(p *model.EnvPreset) error {
	if p == nil {
		return errors.New("env preset is nil")
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidEnvPreset
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.envPresets[p.Name]; exists {
		return ErrEnvPresetExists
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Env == nil {
		p.Env = map[string]string{}
	}
	e.envPresets[p.Name] = p
	return e.saveEnvPreset(p)
}

func (e *Engine) GetEvent(id string) (*model.Event, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ev, exists := e.events[id]
	if !exists {
		return nil, ErrEventNotFound
	}
	clone := *ev
	return &clone, nil
}

func (e *Engine) GetEnvPreset(name string) (*model.EnvPreset, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.envPresets[name]
	if !exists {
		return nil, ErrEnvPresetNotFound
	}
	clone := *p
	return &clone, nil
}

func (e *Engine) UpdateEvent(id string, updates func(*model.Event)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ev, exists := e.events[id]
	if !exists {
		return ErrEventNotFound
	}

	updates(ev)
	ev.UpdatedAt = time.Now()
	return e.saveEvent(ev)
}

func (e *Engine) UpdateEnvPreset(name string, updates func(*model.EnvPreset)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, exists := e.envPresets[name]
	if !exists {
		return ErrEnvPresetNotFound
	}

	updates(p)
	p.UpdatedAt = time.Now()
	if p.Env == nil {
		p.Env = map[string]string{}
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidEnvPreset
	}
	if p.Name != name {
		return ErrInvalidEnvPreset
	}
	return e.saveEnvPreset(p)
}

func (e *Engine) DeleteEvent(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.events[id]; !exists {
		return ErrEventNotFound
	}

	for _, t := range e.triggers {
		var filtered []string
		for _, eid := range t.EventIDs {
			if eid != id {
				filtered = append(filtered, eid)
			}
		}
		if len(filtered) != len(t.EventIDs) {
			t.EventIDs = filtered
			_ = e.saveTrigger(t)
		}
	}

	delete(e.events, id)
	return e.deleteEventFile(id)
}

func (e *Engine) DeleteEnvPreset(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.envPresets[name]; !exists {
		return ErrEnvPresetNotFound
	}
	delete(e.envPresets, name)
	return e.deleteEnvPresetFile(name)
}

func (e *Engine) ListEvents() []model.Event {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]model.Event, 0, len(e.events))
	for _, ev := range e.events {
		list = append(list, *ev)
	}
	return list
}

func (e *Engine) ListEnvPresets() []model.EnvPreset {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]model.EnvPreset, 0, len(e.envPresets))
	for _, p := range e.envPresets {
		list = append(list, *p)
	}
	return list
}

func (e *Engine) ResolveEventEnv(ev *model.Event) (map[string]string, error) {
	if ev == nil {
		return nil, nil
	}

	merged := make(map[string]string)
	if ev.EnvPreset != "" {
		preset, err := e.GetEnvPreset(ev.EnvPreset)
		if err != nil {
			return nil, err
		}
		for key, val := range preset.Env {
			merged[key] = val
		}
	}
	for key, val := range ev.Env {
		merged[key] = val
	}
	return merged, nil
}

func (e *Engine) GetTriggerEvents(triggerID string) ([]model.Event, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, exists := e.triggers[triggerID]
	if !exists {
		return nil, ErrTriggerNotFound
	}

	var result []model.Event
	for _, eid := range t.EventIDs {
		if ev, exists := e.events[eid]; exists {
			result = append(result, *ev)
		}
	}
	return result, nil
}

func (e *Engine) StartTrigger(id string, executor *Executor) error {
	e.mu.RLock()
	t, exists := e.triggers[id]
	if !exists {
		e.mu.RUnlock()
		return ErrTriggerNotFound
	}

	if e.isRunningUnsafe(id) {
		e.mu.RUnlock()
		return fmt.Errorf("trigger '%s' is already running", id)
	}

	events := make([]*model.Event, 0, len(t.EventIDs))
	for _, eid := range t.EventIDs {
		if ev, exists := e.events[eid]; exists {
			events = append(events, ev)
		}
	}
	e.mu.RUnlock()

	var runner Runner
	switch t.Type {
	case model.TriggerTypeDirect:
		runner = newDirectRunner(t)
	case model.TriggerTypePeriodic:
		runner = newPeriodicRunner(t)
	case model.TriggerTypeEdit:
		runner = newEditRunner(t, e.basePath)
	default:
		return ErrInvalidType
	}

	if err := runner.Start(events, executor); err != nil {
		return fmt.Errorf("start trigger: %w", err)
	}

	e.runMu.Lock()
	e.running[id] = runner
	e.runMu.Unlock()

	return nil
}

func (e *Engine) StopTrigger(id string) error {
	e.runMu.Lock()
	defer e.runMu.Unlock()

	runner, exists := e.running[id]
	if !exists {
		return fmt.Errorf("trigger '%s' is not running", id)
	}

	if err := runner.Stop(); err != nil {
		return fmt.Errorf("stop trigger: %w", err)
	}
	delete(e.running, id)
	return nil
}

func (e *Engine) IsRunning(id string) bool {
	e.runMu.RLock()
	defer e.runMu.RUnlock()
	_, exists := e.running[id]
	return exists
}

func (e *Engine) isRunningUnsafe(id string) bool {
	_, exists := e.running[id]
	return exists
}

func (e *Engine) StopAll() {
	e.runMu.Lock()
	defer e.runMu.Unlock()

	for id, runner := range e.running {
		runner.Stop()
		delete(e.running, id)
	}
}

func (e *Engine) Workspace() string {
	return e.workspace
}

func (e *Engine) BasePath() string {
	return e.basePath
}
