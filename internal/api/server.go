package api

import (
	"net/http"
	"time"

	"recipe_manager/internal/auth"
	"recipe_manager/internal/config"
	"recipe_manager/internal/storage"
)

// Server holds the dependencies for all HTTP handlers.
type Server struct {
	db  *storage.DB
	cfg config.Config
}

// New creates a Server and registers all routes on mux.
func New(mux *http.ServeMux, db *storage.DB, cfg config.Config) *Server {
	s := &Server{db: db, cfg: cfg}
	s.registerRoutes(mux)
	return s
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	requireAuth := auth.RequireAuth(s.db, s.cfg.JWTSecret)

	// Auth
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.Handle("GET /auth/me", requireAuth(http.HandlerFunc(s.handleMe)))

	// Recipes (protected)
	mux.Handle("GET /api/recipes", requireAuth(http.HandlerFunc(s.handleListRecipes)))
	mux.Handle("POST /api/recipes", requireAuth(http.HandlerFunc(s.handleCreateRecipe)))
	mux.Handle("GET /api/recipes/{id}", requireAuth(http.HandlerFunc(s.handleGetRecipe)))
	mux.Handle("PUT /api/recipes/{id}", requireAuth(http.HandlerFunc(s.handleUpdateRecipe)))
	mux.Handle("DELETE /api/recipes/{id}", requireAuth(http.HandlerFunc(s.handleDeleteRecipe)))

	// Weekly plan (protected)
	mux.Handle("GET /api/weekly-plan", requireAuth(http.HandlerFunc(s.handleGetWeeklyPlan)))
	mux.Handle("PUT /api/weekly-plan/{id}/items", requireAuth(http.HandlerFunc(s.handleReplaceWeeklyPlanItems)))
	mux.Handle("DELETE /api/weekly-plan/{id}/items/{itemId}", requireAuth(http.HandlerFunc(s.handleDeleteWeeklyPlanItem)))

	// Shopping list (protected)
	mux.Handle("GET /api/shopping-list", requireAuth(http.HandlerFunc(s.handleGetShoppingList)))
	mux.Handle("POST /api/shopping-list/check", requireAuth(http.HandlerFunc(s.handleSetShoppingCheck)))

	// Pantry (protected)
	mux.Handle("GET /api/pantry", requireAuth(http.HandlerFunc(s.handleListPantry)))
	mux.Handle("POST /api/pantry", requireAuth(http.HandlerFunc(s.handleUpsertPantryItem)))
	mux.Handle("DELETE /api/pantry/{id}", requireAuth(http.HandlerFunc(s.handleDeletePantryItem)))

	// Photo upload (protected)
	mux.Handle("POST /api/upload", requireAuth(http.HandlerFunc(s.handleUpload)))

	// Default weekly plan template (protected)
	mux.Handle("GET /api/default-plan", requireAuth(http.HandlerFunc(s.handleGetDefaultPlan)))
	mux.Handle("PUT /api/default-plan/items", requireAuth(http.HandlerFunc(s.handleReplaceDefaultPlanItems)))

	// Import from URL / text / image via Claude (protected)
	mux.Handle("POST /api/import", requireAuth(http.HandlerFunc(s.handleImport)))

	// MCP service token routes
	if s.cfg.MCPServiceToken != "" {
		mux.Handle("GET /api/mcp/recipes", mcpAuth(s.cfg.MCPServiceToken)(http.HandlerFunc(s.handleMCPListRecipes)))
		mux.Handle("POST /api/mcp/recipes", mcpAuth(s.cfg.MCPServiceToken)(http.HandlerFunc(s.handleMCPCreateRecipe)))
		mux.Handle("GET /api/mcp/weekly-plan", mcpAuth(s.cfg.MCPServiceToken)(http.HandlerFunc(s.handleMCPGetWeeklyPlan)))
	}
}

// mcpAuth validates the static MCP service token.
func mcpAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := r.Header.Get("X-MCP-Token")
			if t == "" {
				t = r.URL.Query().Get("token")
			}
			if t != token {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ── Auth handlers ─────────────────────────────────────────────────────────

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Username != s.cfg.Username || body.Password != s.cfg.Password {
		errorJSON(w, http.StatusUnauthorized, "identifiants incorrects")
		return
	}

	// Upsert a fixed user record for this account
	user := &storage.User{
		ID:    "local-user",
		Email: body.Username,
		Name:  body.Username,
	}
	if err := s.db.UpsertUser(user); err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	jwtToken, err := auth.Sign("local-user", s.cfg.JWTSecret, 30*24*time.Hour)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "session",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

// ── MCP-specific handlers ─────────────────────────────────────────────────

func (s *Server) handleMCPListRecipes(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "local-user"
	}
	recipes, err := s.db.ListRecipes(userID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if recipes == nil {
		recipes = []storage.RecipeSummary{}
	}
	writeJSON(w, http.StatusOK, recipes)
}

func (s *Server) handleMCPCreateRecipe(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "local-user"
	}
	var body recipeBody
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	rec := &storage.Recipe{
		Title:        body.Title,
		Description:  body.Description,
		Instructions: body.Instructions,
		Servings:     body.Servings,
		Tags:         body.Tags,
		SourceURL:    body.SourceURL,
		ImageURL:     body.ImageURL,
	}
	created, err := s.db.CreateRecipe(userID, rec, body.Ingredients)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleMCPGetWeeklyPlan(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "local-user"
	}
	week := r.URL.Query().Get("week")
	if week == "" {
		errorJSON(w, http.StatusBadRequest, "week required")
		return
	}
	plan, err := s.db.GetOrCreateWeeklyPlan(userID, week)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}
