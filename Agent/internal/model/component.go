package model

import "time"

type ComponentType string

const (
	ComponentStatic   ComponentType = "static"
	ComponentEmbedded ComponentType = "embedded"
)

type Component struct {
	Name        string        `json:"name"`
	Type        ComponentType `json:"type"`
	Content     string        `json:"content"`
	Description string        `json:"description"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}
