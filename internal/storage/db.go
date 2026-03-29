package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection.
type DB struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database and runs all migrations.
func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite write serialisation
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA foreign_keys=ON;

		CREATE TABLE IF NOT EXISTS users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL UNIQUE,
			name         TEXT NOT NULL,
			picture_url  TEXT,
			created_at   TEXT NOT NULL DEFAULT (datetime('now')),
			last_seen_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS recipes (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title        TEXT NOT NULL,
			description  TEXT,
			instructions TEXT NOT NULL,
			servings     INTEGER NOT NULL DEFAULT 4,
			source_url   TEXT,
			image_url    TEXT,
			created_at   TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_recipes_user ON recipes(user_id);

		CREATE TABLE IF NOT EXISTS ingredients (
			id   TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			unit TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS recipe_ingredients (
			id            TEXT PRIMARY KEY,
			recipe_id     TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
			ingredient_id TEXT NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
			quantity      REAL NOT NULL,
			unit_override TEXT,
			notes         TEXT
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_ri_unique ON recipe_ingredients(recipe_id, ingredient_id);
		CREATE INDEX IF NOT EXISTS idx_ri_recipe ON recipe_ingredients(recipe_id);

		CREATE TABLE IF NOT EXISTS weekly_plans (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			week_start TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, week_start)
		);
		CREATE INDEX IF NOT EXISTS idx_wp_user_week ON weekly_plans(user_id, week_start);

		CREATE TABLE IF NOT EXISTS weekly_plan_items (
			id             TEXT PRIMARY KEY,
			weekly_plan_id TEXT NOT NULL REFERENCES weekly_plans(id) ON DELETE CASCADE,
			recipe_id      TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
			day_of_week    INTEGER NOT NULL CHECK(day_of_week BETWEEN 1 AND 7),
			meal_type      TEXT NOT NULL DEFAULT 'dinner'
			               CHECK(meal_type IN ('breakfast','lunch','dinner','snack')),
			people_count   INTEGER NOT NULL DEFAULT 4
		);
		CREATE INDEX IF NOT EXISTS idx_wpi_plan ON weekly_plan_items(weekly_plan_id);

		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS recipe_tags (
			recipe_id TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
			tag       TEXT NOT NULL,
			PRIMARY KEY (recipe_id, tag)
		);
		CREATE INDEX IF NOT EXISTS idx_recipe_tags_tag ON recipe_tags(tag);

		CREATE TABLE IF NOT EXISTS shopping_checks (
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			week_start TEXT NOT NULL,
			ingredient TEXT NOT NULL,
			checked_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (user_id, week_start, ingredient)
		);

		CREATE TABLE IF NOT EXISTS pantry_items (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			ingredient TEXT NOT NULL,
			quantity   REAL NOT NULL DEFAULT 0,
			unit       TEXT NOT NULL DEFAULT '',
			UNIQUE(user_id, ingredient)
		);
		CREATE INDEX IF NOT EXISTS idx_pantry_user ON pantry_items(user_id);

		CREATE TABLE IF NOT EXISTS default_plan_items (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			day_of_week  INTEGER NOT NULL CHECK(day_of_week BETWEEN 1 AND 7),
			meal_type    TEXT NOT NULL DEFAULT 'dinner'
			             CHECK(meal_type IN ('breakfast','lunch','dinner','snack')),
			recipe_id    TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
			people_count INTEGER NOT NULL DEFAULT 4,
			UNIQUE(user_id, day_of_week, meal_type)
		);
		CREATE INDEX IF NOT EXISTS idx_dpi_user ON default_plan_items(user_id);
	`)
	return err
}
