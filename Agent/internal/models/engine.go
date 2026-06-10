package models

import (
	"errors"
	"sync"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var (
	ErrModelNotFound = errors.New("model not found")
	ErrModelExists   = errors.New("model already exists")
	ErrInvalidName   = errors.New("invalid model name")
)

type Repository interface {
	Get(name string) (*model.Model, error)
	List() ([]model.Model, error)
	Create(m *model.Model) error
	Update(m *model.Model) error
	Delete(name string) error
	Save(m *model.Model) error
}

type Engine struct {
	repo Repository
	mu   sync.RWMutex
}

func NewEngine(repo Repository) *Engine {
	return &Engine{repo: repo}
}

func (e *Engine) Get(name string) (*model.Model, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	m, err := e.repo.Get(name)
	if err != nil {
		return nil, err
	}

	clone := *m
	clone.APIKey = ""
	return &clone, nil
}

func (e *Engine) GetSecure(name string) (*model.Model, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	return e.repo.Get(name)
}

func (e *Engine) List() ([]model.Model, error) {
	list, err := e.repo.List()
	if err != nil {
		return nil, err
	}

	for i := range list {
		list[i].APIKey = ""
	}
	return list, nil
}

func (e *Engine) Create(m *model.Model) error {
	if m == nil {
		return errors.New("model is nil")
	}
	if m.Name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := e.repo.Get(m.Name)
	if err == nil {
		return ErrModelExists
	}
	if !errors.Is(err, ErrModelNotFound) {
		return err
	}

	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now

	return e.repo.Create(m)
}

func (e *Engine) Update(name string, updates *model.Model) error {
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

	if updates.Model != "" {
		existing.Model = updates.Model
	}
	if updates.BaseURL != "" {
		existing.BaseURL = updates.BaseURL
	}
	if updates.APIKey != "" {
		existing.APIKey = updates.APIKey
	}
	if updates.APIType != "" {
		existing.APIType = updates.APIType
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

func (e *Engine) Save(m *model.Model) error {
	if m == nil {
		return errors.New("model is nil")
	}
	if m.Name == "" {
		return ErrInvalidName
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	return e.repo.Save(m)
}
