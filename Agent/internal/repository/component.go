package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/component"
	"github.com/Lyoomu/TAC/Agent/internal/model"
)

type ComponentRepo struct {
	db *sql.DB
}

func NewComponentRepo(db *sql.DB) *ComponentRepo {
	return &ComponentRepo{db: db}
}

func (r *ComponentRepo) Get(name string) (*model.Component, error) {
	row := r.db.QueryRow("SELECT name, type, content, description, created_at, updated_at FROM components WHERE name = ?", name)
	var c model.Component
	var createdAt, updatedAt string
	err := row.Scan(&c.Name, &c.Type, &c.Content, &c.Description, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, component.ErrComponentNotFound
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &c, nil
}

func (r *ComponentRepo) List() ([]model.Component, error) {
	rows, err := r.db.Query("SELECT name, type, content, description, created_at, updated_at FROM components")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Component
	for rows.Next() {
		var c model.Component
		var createdAt, updatedAt string
		if err := rows.Scan(&c.Name, &c.Type, &c.Content, &c.Description, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *ComponentRepo) Create(c *model.Component) error {
	_, err := r.db.Exec(
		"INSERT INTO components (name, type, content, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		c.Name, c.Type, c.Content, c.Description, c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (r *ComponentRepo) Update(c *model.Component) error {
	_, err := r.db.Exec(
		"UPDATE components SET type = ?, content = ?, description = ?, updated_at = ? WHERE name = ?",
		c.Type, c.Content, c.Description, c.UpdatedAt.Format(time.RFC3339), c.Name,
	)
	return err
}

func (r *ComponentRepo) Delete(name string) error {
	_, err := r.db.Exec("DELETE FROM components WHERE name = ?", name)
	return err
}
