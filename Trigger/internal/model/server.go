package model

import "time"

type ServerConnection struct {
	DisplayName        string       `yaml:"display_name" json:"display_name"`               // 用户自定义显示名
	Address            string       `yaml:"address" json:"address"`                         // IP:Port 或域名
	AuthToken          string       `yaml:"auth_token" json:"auth_token"`                   // 鉴权 token
	TrustedFingerprint string       `yaml:"trusted_fingerprint" json:"trusted_fingerprint"` // 预配置的证书指纹
	IsActive           bool         `yaml:"is_active" json:"is_active"`
	Roles              []LoadedRole `yaml:"roles" json:"roles"` // 已加载的 Role
	CreatedAt          time.Time    `yaml:"created_at" json:"created_at"`
	UpdatedAt          time.Time    `yaml:"updated_at" json:"updated_at"`
}

type LoadedRole struct {
	ServerName  string     `yaml:"server_name" json:"server_name"` // 所属 Server 的 display name
	RoleName    string     `yaml:"role_name" json:"role_name"`
	Description string     `yaml:"description" json:"description"`
	APIType     string     `yaml:"api_type" json:"api_type"`
	MessageMode string     `yaml:"message_mode,omitempty" json:"message_mode,omitempty"`
	Tools       []ToolInfo `yaml:"tools" json:"tools"`
	LoadedAt    time.Time  `yaml:"loaded_at" json:"loaded_at"`
}

type ToolInfo struct {
	Name                string    `yaml:"name" json:"name"`
	Description         string    `yaml:"description" json:"description"`
	Language            string    `yaml:"language" json:"language"`
	Version             string    `yaml:"version" json:"version"`
	Dependencies        []string  `yaml:"dependencies" json:"dependencies"`
	RequiresCompilation bool      `yaml:"requires_compilation" json:"requires_compilation"`
	IsBinary            bool      `yaml:"is_binary" json:"is_binary"`
	SourceAvailable     bool      `yaml:"source_available" json:"source_available"`
	RuntimeRequirement  string    `yaml:"runtime_requirement" json:"runtime_requirement"`
	Files               []string  `yaml:"files" json:"files"`
	LocalPath           string    `yaml:"local_path" json:"local_path"` // 本地存储路径
	DownloadedAt        time.Time `yaml:"downloaded_at" json:"downloaded_at"`
}
