package api

import "net/http"

func (s *Server) handleGetShoppingList(w http.ResponseWriter, r *http.Request) {
	week := r.URL.Query().Get("week")
	if week == "" {
		errorJSON(w, http.StatusBadRequest, "week parameter required (YYYY-MM-DD)")
		return
	}
	user := currentUser(r)
	list, err := s.db.GetShoppingList(user.ID, week)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSetShoppingCheck(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var body struct {
		Week       string `json:"week"`
		Ingredient string `json:"ingredient"`
		Checked    bool   `json:"checked"`
	}
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Week == "" || body.Ingredient == "" {
		errorJSON(w, http.StatusBadRequest, "week and ingredient are required")
		return
	}
	if err := s.db.SetShoppingCheck(user.ID, body.Week, body.Ingredient, body.Checked); err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
