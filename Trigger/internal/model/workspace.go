package model

import "time"

type Workspace struct {
	Name      string    `json:"name" yaml:"name"`           // 逻辑目录名
	Path      string    `json:"path" yaml:"path"`           // 真实磁盘目录（绝对路径）
	IsActive  bool      `json:"is_active" yaml:"is_active"` // 是否当前激活
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}
