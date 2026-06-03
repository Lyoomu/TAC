package model

import "time"

type Model struct {
	Name      string    `json:"name"`
	Model     string    `json:"model"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key,omitempty"`
	APIType   APIType   `json:"api_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
