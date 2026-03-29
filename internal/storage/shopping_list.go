package storage

import "strings"

// ShoppingItem is one aggregated line in the shopping list.
type ShoppingItem struct {
	Ingredient    string  `json:"ingredient"`
	Unit          string  `json:"unit"`
	TotalQuantity float64 `json:"total_quantity"`
	PantryQty     float64 `json:"pantry_qty"`
	NeededQty     float64 `json:"needed_qty"`
	Checked       bool    `json:"checked"`
}

// ShoppingList is the full shopping list for a week.
type ShoppingList struct {
	WeekStart string         `json:"week_start"`
	Items     []ShoppingItem `json:"items"`
}

// GetShoppingList aggregates all ingredients for the given week, scaling by people_count,
// then subtracts pantry quantities so only what still needs to be bought is returned.
func (d *DB) GetShoppingList(userID, weekStart string) (*ShoppingList, error) {
	rows, err := d.db.Query(`
		SELECT
			i.name,
			COALESCE(ri.unit_override, i.unit) AS unit,
			SUM(ri.quantity * (CAST(wpi.people_count AS REAL) / CAST(r.servings AS REAL))) AS total_quantity
		FROM weekly_plan_items wpi
		JOIN weekly_plans wp        ON wp.id  = wpi.weekly_plan_id
		JOIN recipes r              ON r.id   = wpi.recipe_id
		JOIN recipe_ingredients ri  ON ri.recipe_id = r.id
		JOIN ingredients i          ON i.id   = ri.ingredient_id
		WHERE wp.user_id = ? AND wp.week_start = ?
		GROUP BY i.id, i.name, unit
		ORDER BY i.name ASC`,
		userID, weekStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := &ShoppingList{WeekStart: weekStart}
	for rows.Next() {
		var item ShoppingItem
		if err := rows.Scan(&item.Ingredient, &item.Unit, &item.TotalQuantity); err != nil {
			return nil, err
		}
		list.Items = append(list.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if list.Items == nil {
		list.Items = []ShoppingItem{}
		return list, nil
	}

	// Fetch pantry — separate query to avoid nested cursor deadlock
	pantryRows, err := d.db.Query(`
		SELECT ingredient, quantity FROM pantry_items WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer pantryRows.Close()
	pantry := make(map[string]float64)
	for pantryRows.Next() {
		var ing string
		var qty float64
		if err := pantryRows.Scan(&ing, &qty); err != nil {
			return nil, err
		}
		pantry[strings.ToLower(ing)] = qty
	}
	if err := pantryRows.Err(); err != nil {
		return nil, err
	}

	// Fetch checked state — separate query to avoid nested cursor deadlock
	checkRows, err := d.db.Query(`
		SELECT ingredient FROM shopping_checks
		WHERE user_id = ? AND week_start = ?`,
		userID, weekStart)
	if err != nil {
		return nil, err
	}
	defer checkRows.Close()
	checked := make(map[string]bool)
	for checkRows.Next() {
		var ing string
		if err := checkRows.Scan(&ing); err != nil {
			return nil, err
		}
		checked[ing] = true
	}

	// Apply pantry deductions and filter out fully-covered items
	filtered := list.Items[:0]
	for _, item := range list.Items {
		pantryQty := pantry[strings.ToLower(item.Ingredient)]
		item.PantryQty = pantryQty
		needed := item.TotalQuantity - pantryQty
		if needed < 0 {
			needed = 0
		}
		item.NeededQty = needed
		item.Checked = checked[item.Ingredient]
		if needed > 0 {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	if list.Items == nil {
		list.Items = []ShoppingItem{}
	}
	return list, nil
}

// SetShoppingCheck marks or unmarks an ingredient as checked for a given week.
func (d *DB) SetShoppingCheck(userID, weekStart, ingredient string, checked bool) error {
	if checked {
		_, err := d.db.Exec(`
			INSERT OR REPLACE INTO shopping_checks (user_id, week_start, ingredient, checked_at)
			VALUES (?, ?, ?, datetime('now'))`,
			userID, weekStart, ingredient)
		return err
	}
	_, err := d.db.Exec(`
		DELETE FROM shopping_checks WHERE user_id=? AND week_start=? AND ingredient=?`,
		userID, weekStart, ingredient)
	return err
}
