package model

import "time"

type Session struct {
	ID          string           `yaml:"id" json:"id"`
	TriggerName string           `yaml:"trigger_name" json:"trigger_name"`
	ServerName  string           `yaml:"server_name" json:"server_name"`
	RoleName    string           `yaml:"role_name" json:"role_name"`
	Messages    []SessionMessage `yaml:"messages" json:"messages"`
	CreatedAt   time.Time        `yaml:"created_at" json:"created_at"`
	UpdatedAt   time.Time        `yaml:"updated_at" json:"updated_at"`
}

type SessionMessage struct {
	Role      string    `yaml:"role" json:"role"` // system, user, assistant
	Content   string    `yaml:"content" json:"content"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

func (s *Session) Key() string {
	return s.ServerName + "-" + s.RoleName + "-" + s.ID
}
