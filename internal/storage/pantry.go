package storage

import "github.com/google/uuid"

// PantryItem is an ingredient the user already has at home.
type PantryItem struct {
	ID         string  `json:"id"`
	Ingredient string  `json:"ingredient"`
	Quantity   float64 `json:"quantity"`
	Unit       string  `json:"unit"`
}

// ListPantry returns all pantry items for a user, sorted by ingredient name.
func (d *DB) ListPantry(userID string) ([]PantryItem, error) {
	rows, err := d.db.Query(`
		SELECT id, ingredient, quantity, unit
		FROM pantry_items
		WHERE user_id = ?
		ORDER BY ingredient ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PantryItem
	for rows.Next() {
		var it PantryItem
		if err := rows.Scan(&it.ID, &it.Ingredient, &it.Quantity, &it.Unit); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = []PantryItem{}
	}
	return items, rows.Err()
}

// UpsertPantryItem inserts or updates a pantry item (matched by user+ingredient name).
func (d *DB) UpsertPantryItem(userID string, item PantryItem) (*PantryItem, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	_, err := d.db.Exec(`
		INSERT INTO pantry_items (id, user_id, ingredient, quantity, unit)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, ingredient) DO UPDATE SET
			quantity = excluded.quantity,
			unit     = excluded.unit,
			id       = id`,
		item.ID, userID, item.Ingredient, item.Quantity, item.Unit)
	if err != nil {
		return nil, err
	}
	// Re-fetch to get the actual id (may differ on conflict)
	var saved PantryItem
	err = d.db.QueryRow(`
		SELECT id, ingredient, quantity, unit FROM pantry_items
		WHERE user_id = ? AND ingredient = ?`, userID, item.Ingredient).
		Scan(&saved.ID, &saved.Ingredient, &saved.Quantity, &saved.Unit)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// DeletePantryItem removes a pantry item by id.
func (d *DB) DeletePantryItem(userID, itemID string) error {
	_, err := d.db.Exec(`DELETE FROM pantry_items WHERE id = ? AND user_id = ?`, itemID, userID)
	return err
}
