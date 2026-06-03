package server

import (
	"context"
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
	ErrServerNotFound    = errors.New("server not found")
	ErrServerExists      = errors.New("server already exists")
	ErrInvalidAddress    = errors.New("invalid server address")
	ErrInvalidName       = errors.New("invalid server name")
	ErrDisplayNameExists = errors.New("display name already exists")
)

type Engine struct {
	servers    map[string]*model.ServerConnection // key = address
	pool       map[string]*Client                 // key = address, live gRPC connections
	configPath string
	mu         sync.RWMutex // protects servers map
	poolMu     sync.Mutex   // protects pool map
	connectMu  sync.Map     // map[string]*sync.Mutex, per-address connect lock
}

func NewEngine() *Engine {
	return &Engine{
		servers:    make(map[string]*model.ServerConnection),
		pool:       make(map[string]*Client),
		configPath: defaultConfigPath(),
	}
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tac", "servers.yaml")
}

func (e *Engine) Load() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := os.Stat(e.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(e.configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg struct {
		Servers []model.ServerConnection `yaml:"servers"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	for _, s := range cfg.Servers {
		s := s
		e.servers[s.Address] = &s
	}
	return nil
}

func (e *Engine) Save() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var cfg struct {
		Servers []model.ServerConnection `yaml:"servers"`
	}
	for _, s := range e.servers {
		cfg.Servers = append(cfg.Servers, *s)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(e.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (e *Engine) Add(address, displayName, authToken, fingerprint string) error {
	if address == "" {
		return ErrInvalidAddress
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.servers[address]; exists {
		return ErrServerExists
	}

	for _, s := range e.servers {
		if s.DisplayName == displayName {
			return ErrDisplayNameExists
		}
	}

	now := time.Now()
	e.servers[address] = &model.ServerConnection{
		DisplayName:        displayName,
		Address:            address,
		AuthToken:          authToken,
		TrustedFingerprint: fingerprint,
		IsActive:           false,
		Roles:              []model.LoadedRole{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return e.saveUnsafe()
}

func (e *Engine) Update(address string, updates func(*model.ServerConnection)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s, exists := e.servers[address]
	if !exists {
		return ErrServerNotFound
	}

	updates(s)
	s.UpdatedAt = time.Now()
	return e.saveUnsafe()
}

func (e *Engine) Remove(address string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.servers[address]; !exists {
		return ErrServerNotFound
	}

	delete(e.servers, address)

	e.poolMu.Lock()
	if c, ok := e.pool[address]; ok {
		c.Close()
		delete(e.pool, address)
	}
	e.poolMu.Unlock()

	return e.saveUnsafe()
}

func (e *Engine) Get(address string) (*model.ServerConnection, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s, exists := e.servers[address]
	if !exists {
		return nil, ErrServerNotFound
	}
	clone := *s
	return &clone, nil
}

func (e *Engine) GetByDisplayName(name string) (*model.ServerConnection, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, s := range e.servers {
		if s.DisplayName == name {
			clone := *s
			return &clone, nil
		}
	}
	return nil, ErrServerNotFound
}

func (e *Engine) List() []model.ServerConnection {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]model.ServerConnection, 0, len(e.servers))
	for _, s := range e.servers {
		list = append(list, *s)
	}
	return list
}

func (e *Engine) Exists(address string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.servers[address]
	return exists
}

func (e *Engine) ExistsByDisplayName(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, s := range e.servers {
		if s.DisplayName == name {
			return true
		}
	}
	return false
}

func (e *Engine) GetClient(address string) (*Client, error) {

	e.poolMu.Lock()
	if c, ok := e.pool[address]; ok {
		e.poolMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := c.Ping(ctx); err == nil {
			return c, nil
		}

	} else {
		e.poolMu.Unlock()
	}

	muIface, _ := e.connectMu.LoadOrStore(address, &sync.Mutex{})
	connMu := muIface.(*sync.Mutex)
	connMu.Lock()
	defer connMu.Unlock()

	e.poolMu.Lock()
	if c, ok := e.pool[address]; ok {
		e.poolMu.Unlock()
		return c, nil
	}
	e.poolMu.Unlock()

	e.mu.RLock()
	meta, exists := e.servers[address]
	e.mu.RUnlock()
	if !exists {
		return nil, ErrServerNotFound
	}

	c, err := Connect(address, meta.AuthToken, meta.TrustedFingerprint)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}

	e.poolMu.Lock()
	e.pool[address] = c
	e.poolMu.Unlock()
	return c, nil
}

func (e *Engine) GetClientByDisplayName(name string) (*Client, error) {
	e.mu.RLock()
	var address string
	for _, s := range e.servers {
		if s.DisplayName == name {
			address = s.Address
			break
		}
	}
	e.mu.RUnlock()

	if address == "" {
		return nil, ErrServerNotFound
	}
	return e.GetClient(address)
}

func (e *Engine) CloseAll() {
	e.poolMu.Lock()
	defer e.poolMu.Unlock()

	for addr, c := range e.pool {
		c.Close()
		delete(e.pool, addr)
	}
}

func (e *Engine) LoadRole(address string, role model.LoadedRole) error {
	return e.Update(address, func(s *model.ServerConnection) {

		for i, r := range s.Roles {
			if r.RoleName == role.RoleName {
				s.Roles[i] = role
				return
			}
		}
		s.Roles = append(s.Roles, role)
	})
}

func (e *Engine) UnloadRole(address, roleName string) error {
	return e.Update(address, func(s *model.ServerConnection) {
		var filtered []model.LoadedRole
		for _, r := range s.Roles {
			if r.RoleName != roleName {
				filtered = append(filtered, r)
			}
		}
		s.Roles = filtered
	})
}

func (e *Engine) GetLoadedRoles() []model.LoadedRole {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []model.LoadedRole
	for _, s := range e.servers {
		for _, r := range s.Roles {
			result = append(result, r)
		}
	}
	return result
}

func (e *Engine) SetTrustedFingerprint(address, fingerprint string) error {
	return e.Update(address, func(s *model.ServerConnection) {
		s.TrustedFingerprint = fingerprint
	})
}

func (e *Engine) saveUnsafe() error {
	var cfg struct {
		Servers []model.ServerConnection `yaml:"servers"`
	}
	for _, s := range e.servers {
		cfg.Servers = append(cfg.Servers, *s)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(e.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
