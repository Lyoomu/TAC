package role

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/component"
	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var (
	ErrRoleNotFound = errors.New("role not found")
	ErrRoleExists   = errors.New("role already exists")
	ErrInvalidName  = errors.New("invalid role name")
)

type Repository interface {
	Get(name string) (*model.Role, error)
	List() ([]model.Role, error)
	Create(r *model.Role) error
	Update(r *model.Role) error
	Delete(name string) error
	Save(r *model.Role) error
}

type Engine struct {
	repo       Repository
	compEngine *component.Engine
	mu         sync.RWMutex
}

func NewEngine(repo Repository, compEngine *component.Engine) *Engine {
	return &Engine{repo: repo, compEngine: compEngine}
}

func (e *Engine) Get(name string) (*model.Role, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	return e.repo.Get(name)
}

func (e *Engine) List() ([]model.Role, error) {
	return e.repo.List()
}

func (e *Engine) Create(r *model.Role) error {
	if r == nil {
		return errors.New("role is nil")
	}
	if r.Name == "" {
		return ErrInvalidName
	}
	if r.MessageMode == "" {
		r.MessageMode = "interrupt"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := e.repo.Get(r.Name)
	if err == nil {
		return ErrRoleExists
	}
	if !errors.Is(err, ErrRoleNotFound) {
		return err
	}

	now := time.Now()
	r.CreatedAt = now
	r.UpdatedAt = now

	return e.repo.Create(r)
}

func (e *Engine) Update(name string, updates *model.Role) error {
	if name == "" {
		return ErrInvalidName
	}
	if updates == nil {
		return errors.New("updates are nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	existing, err := e.repo.Get(name)
	if err != nil {
		return err
	}

	if updates.Description != "" {
		existing.Description = updates.Description
	}
	if updates.Components != nil {
		existing.Components = updates.Components
	}
	if updates.Env != nil {
		existing.Env = updates.Env
	}
	if updates.Needs != nil {
		existing.Needs = updates.Needs
	}
	if updates.Tools != nil {
		existing.Tools = updates.Tools
	}
	if updates.APIType != "" {
		existing.APIType = updates.APIType
	}
	if updates.MessageMode != "" {
		existing.MessageMode = updates.MessageMode
	}
	if updates.Model != "" {
		existing.Model = updates.Model
	}
	existing.UpdatedAt = time.Now()

	return e.repo.Update(existing)
}

func (e *Engine) GetTools(roleName string) ([]string, error) {
	role, err := e.repo.Get(roleName)
	if err != nil {
		return nil, err
	}
	return role.Tools, nil
}

func (e *Engine) Delete(name string) error {
	if name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.repo.Delete(name)
}

func (e *Engine) Save(r *model.Role) error {
	if r == nil {
		return errors.New("role is nil")
	}
	if r.Name == "" {
		return ErrInvalidName
	}
	if r.MessageMode == "" {
		r.MessageMode = "interrupt"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	return e.repo.Save(r)
}

func (e *Engine) AssemblePrompts(roleName string) ([]string, error) {
	role, err := e.repo.Get(roleName)
	if err != nil {
		return nil, err
	}

	var prompts []string
	for _, compName := range role.Components {
		comp, err := e.compEngine.Get(compName)
		if err != nil {
			return nil, err
		}
		content := comp.Content
		if comp.Type == model.ComponentEmbedded {
			content = replacePlaceholders(content, role.Env)
		}
		prompts = append(prompts, content)
	}

	return prompts, nil
}

func replacePlaceholders(content string, env map[string]string) string {
	result := content
	for key, value := range env {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}
	return result
}
