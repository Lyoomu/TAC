package model

import "time"

type APIType string

const (
	APITypeChatCompletion APIType = "chat_completion"
	APITypeResponses      APIType = "responses"
	APITypeAnthropic      APIType = "anthropic"
)

type Need struct {
	Name    string `json:"name"`
	Default string `json:"default"`
}

type Role struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Components  []string          `json:"components"`
	Tools       []string          `json:"tools"`
	Env         map[string]string `json:"env"`
	Needs       []Need            `json:"needs"`
	APIType     APIType           `json:"api_type"`
	MessageMode string            `json:"message_mode"`
	Model       string            `json:"model"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
