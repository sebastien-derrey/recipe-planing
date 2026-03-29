package api

import (
	"net/http"

	"recipe_manager/internal/storage"
)

func (s *Server) handleGetWeeklyPlan(w http.ResponseWriter, r *http.Request) {
	week := r.URL.Query().Get("week")
	if week == "" {
		errorJSON(w, http.StatusBadRequest, "week parameter required (YYYY-MM-DD)")
		return
	}
	user := currentUser(r)
	plan, err := s.db.GetOrCreateWeeklyPlan(user.ID, week)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) handleReplaceWeeklyPlanItems(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("id")
	var inputs []storage.WeeklyPlanItemInput
	if err := readJSON(r, &inputs); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	items, err := s.db.ReplaceWeeklyPlanItems(planID, inputs)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []storage.WeeklyPlanItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteWeeklyPlanItem(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("id")
	itemID := r.PathValue("itemId")
	ok, err := s.db.DeleteWeeklyPlanItem(itemID, planID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		errorJSON(w, http.StatusNotFound, "item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
