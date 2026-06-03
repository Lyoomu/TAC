package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Migration struct {
	Name string
	Up   string
	Down string
}

var Migrations = []Migration{
	{
		Name: "001_create_migrations_table",
		Up: `CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		Down: "DELETE FROM migrations;",
	},
	{
		Name: "002_create_components_table",
		Up: `CREATE TABLE IF NOT EXISTS components (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('static', 'embedded')),
			content TEXT NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		Down: "DROP TABLE IF EXISTS components;",
	},
	{
		Name: "003_create_models_table",
		Up: `CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			base_url TEXT NOT NULL,
			api_key TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		Down: "DROP TABLE IF EXISTS models;",
	},
	{
		Name: "004_create_roles_table",
		Up: `CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			components TEXT,
			tools TEXT,
			env TEXT,
			needs TEXT,
			api_type TEXT,
			message_mode TEXT DEFAULT 'interrupt',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		Down: "DROP TABLE IF EXISTS roles;",
	},
	{
		Name: "005_add_role_api_type",
		Up:   "ALTER TABLE roles ADD COLUMN api_type TEXT;",
		Down: "",
	},
	{
		Name: "006_add_model_model_field",
		Up:   "ALTER TABLE models ADD COLUMN model TEXT;",
		Down: "",
	},
	{
		Name: "007_add_role_message_mode",
		Up:   "ALTER TABLE roles ADD COLUMN message_mode TEXT DEFAULT 'interrupt';",
		Down: "",
	},
	{
		Name: "008_add_role_model_field",
		Up:   "ALTER TABLE roles ADD COLUMN model TEXT;",
		Down: "",
	},
	{
		Name: "009_create_tools_table",
		Up: `CREATE TABLE IF NOT EXISTS tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			type TEXT NOT NULL DEFAULT 'function',
			parameters TEXT NOT NULL DEFAULT '{"type":"object","properties":{},"required":[]}',
			strict INTEGER NOT NULL DEFAULT 0,
			version TEXT,
			script_type TEXT NOT NULL CHECK(script_type IN ('win', 'linux', 'python', 'javascripts', 'typescripts')),
			script_dir TEXT,
			dependencies TEXT,
			requires_compilation INTEGER NOT NULL DEFAULT 0,
			is_binary INTEGER NOT NULL DEFAULT 0,
			source_available INTEGER NOT NULL DEFAULT 1,
			runtime_requirement TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		Down: "DROP TABLE IF EXISTS tools;",
	},
}

func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func isAlterAddColumn(sql string) (table, column string, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if !strings.HasPrefix(upper, "ALTER TABLE ") {
		return "", "", false
	}
	if !strings.Contains(upper, "ADD COLUMN") {
		return "", "", false
	}

	parts := strings.Fields(strings.TrimSpace(sql))
	if len(parts) < 6 {
		return "", "", false
	}
	return parts[2], parts[5], true
}

func MigrateUp(db *sql.DB) error {
	for _, m := range Migrations {
		var exists int
		err := db.QueryRow("SELECT 1 FROM migrations WHERE name = ?", m.Name).Scan(&exists)
		if err == nil {
			continue
		}

		if table, column, ok := isAlterAddColumn(m.Up); ok {
			if columnExists(db, table, column) {

				if _, err := db.Exec("INSERT INTO migrations (name) VALUES (?)", m.Name); err != nil {
					return fmt.Errorf("record migration %s: %w", m.Name, err)
				}
				fmt.Printf("skipped (column exists): %s\n", m.Name)
				continue
			}
		}

		if _, err := db.Exec(m.Up); err != nil {
			return fmt.Errorf("migrate up %s: %w", m.Name, err)
		}
		if _, err := db.Exec("INSERT INTO migrations (name) VALUES (?)", m.Name); err != nil {
			return fmt.Errorf("record migration %s: %w", m.Name, err)
		}
		fmt.Printf("applied: %s\n", m.Name)
	}
	return nil
}

func MigrateDown(db *sql.DB) error {
	for i := len(Migrations) - 1; i >= 0; i-- {
		m := Migrations[i]
		var exists int
		err := db.QueryRow("SELECT 1 FROM migrations WHERE name = ?", m.Name).Scan(&exists)
		if err != nil {
			continue
		}

		if _, err := db.Exec(m.Down); err != nil {
			return fmt.Errorf("migrate down %s: %w", m.Name, err)
		}
		if _, err := db.Exec("DELETE FROM migrations WHERE name = ?", m.Name); err != nil {
			return fmt.Errorf("record migration %s: %w", m.Name, err)
		}
		fmt.Printf("rolled back: %s\n", m.Name)
	}
	return nil
}
