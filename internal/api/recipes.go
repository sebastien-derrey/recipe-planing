package api

import (
	"net/http"

	"recipe_manager/internal/storage"
)

type recipeBody struct {
	Title        string                    `json:"title"`
	Description  string                    `json:"description"`
	Instructions string                    `json:"instructions"`
	Servings     int                       `json:"servings"`
	Tags         []string                  `json:"tags"`
	SourceURL    string                    `json:"source_url"`
	ImageURL     string                    `json:"image_url"`
	Ingredients  []storage.IngredientInput `json:"ingredients"`
}

func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	recipes, err := s.db.ListRecipes(user.ID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if recipes == nil {
		recipes = []storage.RecipeSummary{}
	}
	writeJSON(w, http.StatusOK, recipes)
}

func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	var body recipeBody
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" || body.Instructions == "" {
		errorJSON(w, http.StatusBadRequest, "title and instructions are required")
		return
	}
	user := currentUser(r)
	rec := &storage.Recipe{
		Title:        body.Title,
		Description:  body.Description,
		Instructions: body.Instructions,
		Servings:     body.Servings,
		Tags:         body.Tags,
		SourceURL:    body.SourceURL,
		ImageURL:     body.ImageURL,
	}
	created, err := s.db.CreateRecipe(user.ID, rec, body.Ingredients)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := currentUser(r)
	recipe, err := s.db.GetRecipe(id, user.ID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if recipe == nil {
		errorJSON(w, http.StatusNotFound, "recipe not found")
		return
	}
	writeJSON(w, http.StatusOK, recipe)
}

func (s *Server) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body recipeBody
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	user := currentUser(r)
	rec := &storage.Recipe{
		Title:        body.Title,
		Description:  body.Description,
		Instructions: body.Instructions,
		Servings:     body.Servings,
		Tags:         body.Tags,
		SourceURL:    body.SourceURL,
		ImageURL:     body.ImageURL,
	}
	updated, err := s.db.UpdateRecipe(id, user.ID, rec, body.Ingredients)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if updated == nil {
		errorJSON(w, http.StatusNotFound, "recipe not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := currentUser(r)
	ok, err := s.db.DeleteRecipe(id, user.ID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		errorJSON(w, http.StatusNotFound, "recipe not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
