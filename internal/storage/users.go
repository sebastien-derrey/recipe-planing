package storage

import (
	"database/sql"
	"time"
)

// User represents an authenticated user.
type User struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"picture_url,omitempty"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

// UpsertUser creates or updates a user record (keyed on Google sub ID).
func (d *DB) UpsertUser(u *User) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`
		INSERT INTO users (id, email, name, picture_url, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email        = excluded.email,
			name         = excluded.name,
			picture_url  = excluded.picture_url,
			last_seen_at = ?`,
		u.ID, u.Email, u.Name, u.PictureURL, now, now, now)
	return err
}

// GetUser returns a user by ID. Returns nil if not found.
func (d *DB) GetUser(id string) (*User, error) {
	u := &User{}
	err := d.db.QueryRow(`
		SELECT id, email, name, COALESCE(picture_url,''), created_at, last_seen_at
		FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.PictureURL, &u.CreatedAt, &u.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}
