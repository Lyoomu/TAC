package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

type Manager struct {
	sessionsDir string
	mu          sync.RWMutex
}

func NewManager() *Manager {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Manager{
		sessionsDir: filepath.Join(home, ".tac", "sessions"),
	}
}

func (m *Manager) sessionFilePath(triggerName, roleName, sessionID string) string {
	if triggerName == "" {
		triggerName = "default"
	}
	tName := sanitizeFilename(triggerName)
	rName := sanitizeFilename(roleName)
	sID := sanitizeFilename(sessionID)
	return filepath.Join(m.sessionsDir, tName, rName, sID+".yaml")
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

func (m *Manager) Save(session *model.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.sessionFilePath(session.TriggerName, session.RoleName, session.ID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	data, err := yaml.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

func (m *Manager) Load(triggerName, roleName, sessionID string) (*model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := m.sessionFilePath(triggerName, roleName, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session model.Session
	if err := yaml.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}
	return &session, nil
}

func (m *Manager) LoadAll() ([]model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []model.Session
	err := filepath.WalkDir(m.sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var session model.Session
		if err := yaml.Unmarshal(data, &session); err != nil {
			return nil
		}
		sessions = append(sessions, session)
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("walk sessions dir: %w", err)
	}
	return sessions, nil
}

func (m *Manager) Delete(triggerName, roleName, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.sessionFilePath(triggerName, roleName, sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

func (m *Manager) Create(triggerName, serverName, roleName string) *model.Session {
	now := time.Now()
	if triggerName == "" {
		triggerName = "default"
	}
	return &model.Session{
		ID:          fmt.Sprintf("%d", now.Unix()),
		TriggerName: triggerName,
		ServerName:  serverName,
		RoleName:    roleName,
		Messages:    []model.SessionMessage{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (m *Manager) AppendMessage(session *model.Session, role, content string) error {
	session.Messages = append(session.Messages, model.SessionMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	session.UpdatedAt = time.Now()
	return m.Save(session)
}

func (m *Manager) GetOrCreate(triggerName, serverName, roleName, sessionID string) (*model.Session, error) {
	if sessionID != "" {
		if session, err := m.Load(triggerName, roleName, sessionID); err == nil {
			return session, nil
		}
	}
	return m.Create(triggerName, serverName, roleName), nil
}
