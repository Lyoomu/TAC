package component

import (
	"errors"
	"sync"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var (
	ErrComponentNotFound = errors.New("component not found")
	ErrComponentExists   = errors.New("component already exists")
	ErrInvalidName       = errors.New("invalid component name")
)

type Repository interface {
	Get(name string) (*model.Component, error)
	List() ([]model.Component, error)
	Create(c *model.Component) error
	Update(c *model.Component) error
	Delete(name string) error
}

type Engine struct {
	repo Repository
	mu   sync.RWMutex
}

func NewEngine(repo Repository) *Engine {
	return &Engine{repo: repo}
}

func (e *Engine) Get(name string) (*model.Component, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	return e.repo.Get(name)
}

func (e *Engine) List() ([]model.Component, error) {
	return e.repo.List()
}

func (e *Engine) Create(c *model.Component) error {
	if c == nil {
		return errors.New("component is nil")
	}
	if c.Name == "" {
		return ErrInvalidName
	}
	if c.Type != model.ComponentStatic && c.Type != model.ComponentEmbedded {
		return errors.New("invalid component type")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := e.repo.Get(c.Name)
	if err == nil {
		return ErrComponentExists
	}
	if !errors.Is(err, ErrComponentNotFound) {
		return err
	}

	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	return e.repo.Create(c)
}

func (e *Engine) Update(name string, updates *model.Component) error {
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

	if updates.Type != "" {
		if updates.Type != model.ComponentStatic && updates.Type != model.ComponentEmbedded {
			return errors.New("invalid component type")
		}
		existing.Type = updates.Type
	}
	if updates.Content != "" {
		existing.Content = updates.Content
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	existing.UpdatedAt = time.Now()

	return e.repo.Update(existing)
}

func (e *Engine) Delete(name string) error {
	if name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.repo.Delete(name)
}
