package model

import "encoding/json"

type ToolConfig struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

type Tool struct {
	Name       string
	Version    string
	Config     ToolConfig
	ConfigJSON json.RawMessage
	ScriptDir  string
	Scripts    []string
	MainFile   string

	Language            string
	Dependencies        []string
	RequiresCompilation bool
	IsBinary            bool
	SourceAvailable     bool
	RuntimeRequirement  string
}
