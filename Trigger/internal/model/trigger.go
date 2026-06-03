package model

import "time"

type TriggerType string

const (
	TriggerTypeDirect   TriggerType = "direct"
	TriggerTypePeriodic TriggerType = "periodic"
	TriggerTypeEdit     TriggerType = "edit"
)

type SessionMode string

const (
	SessionModeShared SessionMode = "shared" // 维护一个 session，支持打断
	SessionModeNew    SessionMode = "new"    // 每次新建 session
)

type Trigger struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Type        TriggerType `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Workspace   string      `yaml:"workspace" json:"workspace"` // 所属 workspace 名称
	Enabled     bool        `yaml:"enabled" json:"enabled"`
	CreatedAt   time.Time   `yaml:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `yaml:"updated_at" json:"updated_at"`

	Interval     string     `yaml:"interval,omitempty" json:"interval,omitempty"`             // time.Duration 格式，如 "5m"
	CronExpr     string     `yaml:"cron_expr,omitempty" json:"cron_expr,omitempty"`           // cron 表达式
	FirstRunTime *time.Time `yaml:"first_run_time,omitempty" json:"first_run_time,omitempty"` // 首次运行时间

	WatchPath string   `yaml:"watch_path,omitempty" json:"watch_path,omitempty"` // 监听的文件/文件夹路径（相对于 workspace 根目录）
	Recursive bool     `yaml:"recursive,omitempty" json:"recursive,omitempty"`   // 是否递归监听子文件夹
	Blacklist []string `yaml:"blacklist,omitempty" json:"blacklist,omitempty"`   // 黑名单 glob 模式

	EventIDs []string `yaml:"event_ids" json:"event_ids"`
}

type Event struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Workspace   string `yaml:"workspace" json:"workspace"`

	RoleKey     string            `yaml:"role_key" json:"role_key"`                             // ServerName-RoleName
	InitialMsg  string            `yaml:"initial_msg" json:"initial_msg"`                       // 初始消息模板（支持占位符）
	Env         map[string]string `yaml:"env" json:"env"`                                       // 环境变量覆盖（支持 file:path:N 绑定）
	EnvPreset   string            `yaml:"env_preset,omitempty" json:"env_preset,omitempty"`     // 关联的 Env 预设名称
	SessionMode SessionMode       `yaml:"session_mode" json:"session_mode"`                     // "shared" 或 "new"
	MessageMode string            `yaml:"message_mode,omitempty" json:"message_mode,omitempty"` // queue, reject, interrupt（空则使用 Role 默认值）

	SharedSessionID string `yaml:"shared_session_id,omitempty" json:"shared_session_id,omitempty"`

	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`
}
