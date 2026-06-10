package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/role"
)

type RoleRepo struct {
	db *sql.DB
}

func NewRoleRepo(db *sql.DB) *RoleRepo {
	return &RoleRepo{db: db}
}

func (r *RoleRepo) Get(name string) (*model.Role, error) {
	row := r.db.QueryRow("SELECT name, description, components, tools, env, needs, api_type, message_mode, model, created_at, updated_at FROM roles WHERE name = ?", name)
	var roleObj model.Role
	var desc, componentsJSON, toolsJSON, envJSON, needsJSON, apiType, messageMode, modelVal, createdAt, updatedAt sql.NullString
	err := row.Scan(&roleObj.Name, &desc, &componentsJSON, &toolsJSON, &envJSON, &needsJSON, &apiType, &messageMode, &modelVal, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, role.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	roleObj.Description = desc.String
	if componentsJSON.Valid && componentsJSON.String != "" {
		if err := json.Unmarshal([]byte(componentsJSON.String), &roleObj.Components); err != nil {
			return nil, fmt.Errorf("unmarshal components for role %s: %w", roleObj.Name, err)
		}
	}
	if toolsJSON.Valid && toolsJSON.String != "" {
		if err := json.Unmarshal([]byte(toolsJSON.String), &roleObj.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal tools for role %s: %w", roleObj.Name, err)
		}
	}
	if envJSON.Valid && envJSON.String != "" {
		if err := json.Unmarshal([]byte(envJSON.String), &roleObj.Env); err != nil {
			return nil, fmt.Errorf("unmarshal env for role %s: %w", roleObj.Name, err)
		}
	}
	if needsJSON.Valid && needsJSON.String != "" {
		if err := json.Unmarshal([]byte(needsJSON.String), &roleObj.Needs); err != nil {
			return nil, fmt.Errorf("unmarshal needs for role %s: %w", roleObj.Name, err)
		}
	}
	roleObj.APIType = model.APIType(apiType.String)
	roleObj.MessageMode = messageMode.String
	roleObj.Model = modelVal.String
	roleObj.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	roleObj.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	return &roleObj, nil
}

func (r *RoleRepo) List() ([]model.Role, error) {
	rows, err := r.db.Query("SELECT name, description, components, tools, env, needs, api_type, message_mode, model, created_at, updated_at FROM roles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Role
	for rows.Next() {
		var role model.Role
		var desc, componentsJSON, toolsJSON, envJSON, needsJSON, apiType, messageMode, modelVal, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&role.Name, &desc, &componentsJSON, &toolsJSON, &envJSON, &needsJSON, &apiType, &messageMode, &modelVal, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		role.Description = desc.String
		if componentsJSON.Valid && componentsJSON.String != "" {
			if err := json.Unmarshal([]byte(componentsJSON.String), &role.Components); err != nil {
				return nil, fmt.Errorf("unmarshal components for role %s: %w", role.Name, err)
			}
		}
		if toolsJSON.Valid && toolsJSON.String != "" {
			if err := json.Unmarshal([]byte(toolsJSON.String), &role.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal tools for role %s: %w", role.Name, err)
			}
		}
		if envJSON.Valid && envJSON.String != "" {
			if err := json.Unmarshal([]byte(envJSON.String), &role.Env); err != nil {
				return nil, fmt.Errorf("unmarshal env for role %s: %w", role.Name, err)
			}
		}
		if needsJSON.Valid && needsJSON.String != "" {
			if err := json.Unmarshal([]byte(needsJSON.String), &role.Needs); err != nil {
				return nil, fmt.Errorf("unmarshal needs for role %s: %w", role.Name, err)
			}
		}
		role.APIType = model.APIType(apiType.String)
		role.MessageMode = messageMode.String
		role.Model = modelVal.String
		role.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		role.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		list = append(list, role)
	}
	return list, rows.Err()
}

func (r *RoleRepo) Create(role *model.Role) error {
	componentsJSON, _ := json.Marshal(role.Components)
	toolsJSON, _ := json.Marshal(role.Tools)
	envJSON, _ := json.Marshal(role.Env)
	needsJSON, _ := json.Marshal(role.Needs)
	_, err := r.db.Exec(
		"INSERT INTO roles (name, description, components, tools, env, needs, api_type, message_mode, model, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		role.Name, role.Description, string(componentsJSON), string(toolsJSON), string(envJSON), string(needsJSON), string(role.APIType), role.MessageMode, role.Model,
		role.CreatedAt.Format(time.RFC3339), role.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (r *RoleRepo) Update(role *model.Role) error {
	componentsJSON, _ := json.Marshal(role.Components)
	toolsJSON, _ := json.Marshal(role.Tools)
	envJSON, _ := json.Marshal(role.Env)
	needsJSON, _ := json.Marshal(role.Needs)
	_, err := r.db.Exec(
		"UPDATE roles SET description = ?, components = ?, tools = ?, env = ?, needs = ?, api_type = ?, message_mode = ?, model = ?, updated_at = ? WHERE name = ?",
		role.Description, string(componentsJSON), string(toolsJSON), string(envJSON), string(needsJSON), string(role.APIType), role.MessageMode, role.Model,
		role.UpdatedAt.Format(time.RFC3339), role.Name,
	)
	return err
}

func (r *RoleRepo) Delete(name string) error {
	_, err := r.db.Exec("DELETE FROM roles WHERE name = ?", name)
	return err
}

func (r *RoleRepo) Save(role *model.Role) error {
	componentsJSON, _ := json.Marshal(role.Components)
	toolsJSON, _ := json.Marshal(role.Tools)
	envJSON, _ := json.Marshal(role.Env)
	needsJSON, _ := json.Marshal(role.Needs)
	_, err := r.db.Exec(
		`INSERT INTO roles (name, description, components, tools, env, needs, api_type, message_mode, model, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   description=excluded.description, components=excluded.components,
		   tools=excluded.tools, env=excluded.env, needs=excluded.needs,
		   api_type=excluded.api_type, message_mode=excluded.message_mode,
		   model=excluded.model, updated_at=excluded.updated_at`,
		role.Name, role.Description, string(componentsJSON), string(toolsJSON), string(envJSON), string(needsJSON),
		string(role.APIType), role.MessageMode, role.Model,
		role.CreatedAt.Format(time.RFC3339), role.UpdatedAt.Format(time.RFC3339),
	)
	return err
}
