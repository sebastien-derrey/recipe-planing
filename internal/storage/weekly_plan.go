package storage

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// WeeklyPlan groups plan items for a given week.
type WeeklyPlan struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	WeekStart string           `json:"week_start"`
	CreatedAt string           `json:"created_at"`
	Items     []WeeklyPlanItem `json:"items"`
}

// WeeklyPlanItem is a single recipe slot in a weekly plan.
type WeeklyPlanItem struct {
	ID           string `json:"id"`
	WeeklyPlanID string `json:"weekly_plan_id"`
	RecipeID     string `json:"recipe_id"`
	RecipeTitle  string `json:"recipe_title,omitempty"`
	DayOfWeek    int    `json:"day_of_week"` // 1=Mon … 7=Sun
	MealType     string `json:"meal_type"`
	PeopleCount  int    `json:"people_count"`
}

// WeeklyPlanItemInput is sent by the client when updating items.
type WeeklyPlanItemInput struct {
	RecipeID    string `json:"recipe_id"`
	DayOfWeek   int    `json:"day_of_week"`
	MealType    string `json:"meal_type"`
	PeopleCount int    `json:"people_count"`
}

// GetOrCreateWeeklyPlan returns the plan for the given week, creating it if absent.
func (d *DB) GetOrCreateWeeklyPlan(userID, weekStart string) (*WeeklyPlan, error) {
	plan := &WeeklyPlan{}
	err := d.db.QueryRow(`
		SELECT id, user_id, week_start, created_at
		FROM weekly_plans WHERE user_id=? AND week_start=?`, userID, weekStart).
		Scan(&plan.ID, &plan.UserID, &plan.WeekStart, &plan.CreatedAt)

	if err == sql.ErrNoRows {
		plan.ID = uuid.NewString()
		plan.UserID = userID
		plan.WeekStart = weekStart
		plan.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		_, err = d.db.Exec(`
			INSERT INTO weekly_plans (id, user_id, week_start, created_at) VALUES (?, ?, ?, ?)`,
			plan.ID, userID, weekStart, plan.CreatedAt)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	items, err := d.getWeeklyPlanItems(plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Items = items
	return plan, nil
}

func (d *DB) getWeeklyPlanItems(planID string) ([]WeeklyPlanItem, error) {
	rows, err := d.db.Query(`
		SELECT wpi.id, wpi.weekly_plan_id, wpi.recipe_id, COALESCE(r.title,''),
		       wpi.day_of_week, wpi.meal_type, wpi.people_count
		FROM weekly_plan_items wpi
		LEFT JOIN recipes r ON r.id = wpi.recipe_id
		WHERE wpi.weekly_plan_id = ?
		ORDER BY wpi.day_of_week, wpi.meal_type`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WeeklyPlanItem
	for rows.Next() {
		var it WeeklyPlanItem
		if err := rows.Scan(&it.ID, &it.WeeklyPlanID, &it.RecipeID, &it.RecipeTitle,
			&it.DayOfWeek, &it.MealType, &it.PeopleCount); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReplaceWeeklyPlanItems atomically replaces all items for a plan.
func (d *DB) ReplaceWeeklyPlanItems(planID string, inputs []WeeklyPlanItemInput) ([]WeeklyPlanItem, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM weekly_plan_items WHERE weekly_plan_id=?`, planID); err != nil {
		return nil, err
	}
	for _, inp := range inputs {
		if inp.MealType == "" {
			inp.MealType = "dinner"
		}
		if inp.PeopleCount == 0 {
			inp.PeopleCount = 4
		}
		_, err := tx.Exec(`
			INSERT INTO weekly_plan_items (id, weekly_plan_id, recipe_id, day_of_week, meal_type, people_count)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), planID, inp.RecipeID, inp.DayOfWeek, inp.MealType, inp.PeopleCount)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.getWeeklyPlanItems(planID)
}

// DeleteWeeklyPlanItem removes a single plan item. Returns false if not found.
func (d *DB) DeleteWeeklyPlanItem(itemID, planID string) (bool, error) {
	res, err := d.db.Exec(`DELETE FROM weekly_plan_items WHERE id=? AND weekly_plan_id=?`, itemID, planID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
