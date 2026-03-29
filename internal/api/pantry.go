package api

import (
	"net/http"

	"recipe_manager/internal/storage"
)

func (s *Server) handleListPantry(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	items, err := s.db.ListPantry(user.ID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUpsertPantryItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var item storage.PantryItem
	if err := readJSON(r, &item); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if item.Ingredient == "" {
		errorJSON(w, http.StatusBadRequest, "ingredient is required")
		return
	}
	saved, err := s.db.UpsertPantryItem(user.ID, item)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeletePantryItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id := r.PathValue("id")
	if err := s.db.DeletePantryItem(user.ID, id); err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
