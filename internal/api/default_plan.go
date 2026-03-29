package api

import (
	"net/http"

	"recipe_manager/internal/storage"
)

func (s *Server) handleGetDefaultPlan(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := s.db.ListDefaultPlan(user.ID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleReplaceDefaultPlanItems(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var inputs []storage.WeeklyPlanItemInput
	if err := readJSON(r, &inputs); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	items, err := s.db.ReplaceDefaultPlanItems(user.ID, inputs)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}
