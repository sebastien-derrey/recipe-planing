package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RecipeIngredient is an ingredient line within a recipe.
type RecipeIngredient struct {
	ID           string  `json:"id"`
	IngredientID string  `json:"ingredient_id"`
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Notes        string  `json:"notes,omitempty"`
}

// Recipe is the full recipe with its ingredients.
type Recipe struct {
	ID           string             `json:"id"`
	UserID       string             `json:"user_id"`
	Title        string             `json:"title"`
	Description  string             `json:"description,omitempty"`
	Instructions string             `json:"instructions"`
	Servings     int                `json:"servings"`
	Tags         []string           `json:"tags"`
	SourceURL    string             `json:"source_url,omitempty"`
	ImageURL     string             `json:"image_url,omitempty"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	Ingredients  []RecipeIngredient `json:"ingredients"`
}

// RecipeSummary is a lightweight row used for list views.
type RecipeSummary struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Servings    int      `json:"servings"`
	Tags        []string `json:"tags"`
	ImageURL    string   `json:"image_url,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

// IngredientInput is the client-supplied ingredient line when creating/updating.
type IngredientInput struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Notes    string  `json:"notes,omitempty"`
}

// getTagsForRecipe returns all tags for a given recipe ID.
func (d *DB) getTagsForRecipe(recipeID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT tag FROM recipe_tags WHERE recipe_id = ? ORDER BY tag ASC`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, rows.Err()
}

// replaceTags atomically replaces all tags for a recipe within a transaction.
func replaceTags(tx *sql.Tx, recipeID string, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM recipe_tags WHERE recipe_id = ?`, recipeID); err != nil {
		return err
	}
	for _, raw := range tags {
		tag := strings.TrimSpace(strings.ToLower(raw))
		if tag == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO recipe_tags (recipe_id, tag) VALUES (?, ?)`, recipeID, tag); err != nil {
			return err
		}
	}
	return nil
}

// ListRecipes returns all recipes belonging to userID.
func (d *DB) ListRecipes(userID string) ([]RecipeSummary, error) {
	rows, err := d.db.Query(`
		SELECT id, title, COALESCE(description,''), servings, COALESCE(image_url,''), created_at
		FROM recipes WHERE user_id = ? ORDER BY title ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecipeSummary
	for rows.Next() {
		var r RecipeSummary
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.Servings, &r.ImageURL, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Tags = []string{}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Fetch all tags in one query to avoid nested cursor deadlock on single SQLite connection
	if len(out) > 0 {
		tagRows, err := d.db.Query(`
			SELECT recipe_id, tag FROM recipe_tags
			WHERE recipe_id IN (SELECT id FROM recipes WHERE user_id = ?)
			ORDER BY tag ASC`, userID)
		if err != nil {
			return nil, err
		}
		defer tagRows.Close()
		tagMap := make(map[string][]string)
		for tagRows.Next() {
			var rid, tag string
			if err := tagRows.Scan(&rid, &tag); err != nil {
				return nil, err
			}
			tagMap[rid] = append(tagMap[rid], tag)
		}
		for i := range out {
			if t, ok := tagMap[out[i].ID]; ok {
				out[i].Tags = t
			}
		}
	}
	return out, nil
}

// GetRecipe returns the full recipe with its ingredients. Returns nil if not found.
func (d *DB) GetRecipe(id, userID string) (*Recipe, error) {
	r := &Recipe{}
	err := d.db.QueryRow(`
		SELECT id, user_id, title, COALESCE(description,''), instructions,
		       servings, COALESCE(source_url,''), COALESCE(image_url,''), created_at, updated_at
		FROM recipes WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&r.ID, &r.UserID, &r.Title, &r.Description, &r.Instructions,
			&r.Servings, &r.SourceURL, &r.ImageURL, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ingrs, err := d.getRecipeIngredients(r.ID)
	if err != nil {
		return nil, err
	}
	r.Ingredients = ingrs
	// getTagsForRecipe is safe here — no open cursor at this point
	tags, err := d.getTagsForRecipe(r.ID)
	if err != nil {
		return nil, err
	}
	r.Tags = tags
	return r, nil
}

func (d *DB) getRecipeIngredients(recipeID string) ([]RecipeIngredient, error) {
	rows, err := d.db.Query(`
		SELECT ri.id, ri.ingredient_id, i.name,
		       ri.quantity, COALESCE(ri.unit_override, i.unit), COALESCE(ri.notes,'')
		FROM recipe_ingredients ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE ri.recipe_id = ?
		ORDER BY i.name ASC`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecipeIngredient
	for rows.Next() {
		var ri RecipeIngredient
		if err := rows.Scan(&ri.ID, &ri.IngredientID, &ri.Name, &ri.Quantity, &ri.Unit, &ri.Notes); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}

// FindByURLOrTitle returns the first recipe matching source_url (if non-empty)
// or title (case-insensitive) for the given user. Returns nil if none found.
func (d *DB) FindByURLOrTitle(userID, sourceURL, title string) (*Recipe, error) {
	var id string
	var err error
	if sourceURL != "" {
		err = d.db.QueryRow(
			`SELECT id FROM recipes WHERE user_id = ? AND source_url = ? LIMIT 1`,
			userID, sourceURL,
		).Scan(&id)
	}
	if (err != nil || id == "") && title != "" {
		err = d.db.QueryRow(
			`SELECT id FROM recipes WHERE user_id = ? AND lower(title) = lower(?) LIMIT 1`,
			userID, title,
		).Scan(&id)
	}
	if err != nil || id == "" {
		return nil, nil
	}
	return d.GetRecipe(id, userID)
}

// CreateRecipe inserts a new recipe together with its ingredient lines and tags.
func (d *DB) CreateRecipe(userID string, r *Recipe, inputs []IngredientInput) (*Recipe, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	r.ID = uuid.NewString()
	r.UserID = userID
	now := time.Now().UTC().Format(time.RFC3339)
	r.CreatedAt = now
	r.UpdatedAt = now

	if r.Servings == 0 {
		r.Servings = 4
	}

	_, err = tx.Exec(`
		INSERT INTO recipes (id, user_id, title, description, instructions, servings, source_url, image_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, userID, r.Title, r.Description, r.Instructions,
		r.Servings, r.SourceURL, r.ImageURL, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert recipe: %w", err)
	}

	if err := insertIngredients(tx, r.ID, inputs); err != nil {
		return nil, err
	}
	if err := replaceTags(tx, r.ID, r.Tags); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetRecipe(r.ID, userID)
}

// UpdateRecipe replaces a recipe's metadata and ingredient lines.
func (d *DB) UpdateRecipe(id, userID string, r *Recipe, inputs []IngredientInput) (*Recipe, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.Exec(`
		UPDATE recipes SET title=?, description=?, instructions=?, servings=?,
		    source_url=?, image_url=?, updated_at=?
		WHERE id=? AND user_id=?`,
		r.Title, r.Description, r.Instructions, r.Servings,
		r.SourceURL, r.ImageURL, now, id, userID)
	if err != nil {
		return nil, fmt.Errorf("update recipe: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil // not found or not owned
	}

	_, err = tx.Exec(`DELETE FROM recipe_ingredients WHERE recipe_id = ?`, id)
	if err != nil {
		return nil, err
	}
	if err := insertIngredients(tx, id, inputs); err != nil {
		return nil, err
	}
	if err := replaceTags(tx, id, r.Tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetRecipe(id, userID)
}

// DeleteRecipe removes a recipe (cascades to recipe_ingredients).
func (d *DB) DeleteRecipe(id, userID string) (bool, error) {
	res, err := d.db.Exec(`DELETE FROM recipes WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// insertIngredients upserts ingredient definitions and inserts join rows.
// Duplicate ingredient names within the same recipe are merged by summing quantities.
func insertIngredients(tx *sql.Tx, recipeID string, inputs []IngredientInput) error {
	// Deduplicate by normalised name, summing quantities.
	type merged struct {
		quantity float64
		unit     string
		notes    string
	}
	seen := make(map[string]*merged)
	order := []string{}
	for _, inp := range inputs {
		name := strings.TrimSpace(strings.ToLower(inp.Name))
		if name == "" {
			continue
		}
		if m, ok := seen[name]; ok {
			m.quantity += inp.Quantity
		} else {
			seen[name] = &merged{quantity: inp.Quantity, unit: inp.Unit, notes: inp.Notes}
			order = append(order, name)
		}
	}

	for _, name := range order {
		m := seen[name]
		// Upsert ingredient definition
		ingID := ""
		err := tx.QueryRow(`SELECT id FROM ingredients WHERE name = ?`, name).Scan(&ingID)
		if err == sql.ErrNoRows {
			ingID = uuid.NewString()
			if _, err = tx.Exec(`INSERT INTO ingredients (id, name, unit) VALUES (?, ?, ?)`,
				ingID, name, m.unit); err != nil {
				return fmt.Errorf("insert ingredient %q: %w", name, err)
			}
		} else if err != nil {
			return err
		}

		riID := uuid.NewString()
		unitOverride := sql.NullString{String: m.unit, Valid: m.unit != ""}
		_, err = tx.Exec(`
			INSERT INTO recipe_ingredients (id, recipe_id, ingredient_id, quantity, unit_override, notes)
			VALUES (?, ?, ?, ?, ?, ?)`,
			riID, recipeID, ingID, m.quantity, unitOverride, m.notes)
		if err != nil {
			return fmt.Errorf("insert recipe_ingredient: %w", err)
		}
	}
	return nil
}
