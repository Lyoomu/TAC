package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/crypto"
	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/models"
)

type ModelRepo struct {
	db        *sql.DB
	encrypter *crypto.Encrypter
}

func NewModelRepo(db *sql.DB, encrypter *crypto.Encrypter) *ModelRepo {
	return &ModelRepo{db: db, encrypter: encrypter}
}

func (r *ModelRepo) Get(name string) (*model.Model, error) {
	row := r.db.QueryRow("SELECT name, model, base_url, api_key, created_at, updated_at FROM models WHERE name = ?", name)
	var m model.Model
	var modelVal sql.NullString
	var encryptedKey string
	var createdAt, updatedAt string
	err := row.Scan(&m.Name, &modelVal, &m.BaseURL, &encryptedKey, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrModelNotFound
	}
	if err != nil {
		return nil, err
	}
	m.Model = modelVal.String
	m.APIKey, err = r.encrypter.Decrypt(encryptedKey)
	if err != nil {
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &m, nil
}

func (r *ModelRepo) List() ([]model.Model, error) {
	rows, err := r.db.Query("SELECT name, model, base_url, api_key, created_at, updated_at FROM models")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Model
	for rows.Next() {
		var m model.Model
		var modelVal sql.NullString
		var encryptedKey string
		var createdAt, updatedAt string
		if err := rows.Scan(&m.Name, &modelVal, &m.BaseURL, &encryptedKey, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		m.Model = modelVal.String
		m.APIKey, err = r.encrypter.Decrypt(encryptedKey)
		if err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *ModelRepo) Create(m *model.Model) error {
	encryptedKey, err := r.encrypter.Encrypt(m.APIKey)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		"INSERT INTO models (name, model, base_url, api_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		m.Name, m.Model, m.BaseURL, encryptedKey, m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (r *ModelRepo) Update(m *model.Model) error {
	encryptedKey, err := r.encrypter.Encrypt(m.APIKey)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		"UPDATE models SET model = ?, base_url = ?, api_key = ?, updated_at = ? WHERE name = ?",
		m.Model, m.BaseURL, encryptedKey, m.UpdatedAt.Format(time.RFC3339), m.Name,
	)
	return err
}

func (r *ModelRepo) Delete(name string) error {
	_, err := r.db.Exec("DELETE FROM models WHERE name = ?", name)
	return err
}
