package api

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"recipe_manager/internal/mcp"
	"recipe_manager/internal/storage"
)

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AnthropicAPIKey == "" {
		errorJSON(w, http.StatusServiceUnavailable, "clé Anthropic non configurée — ajoutez anthropic_api_key dans config.json")
		return
	}
	user := currentUser(r)

	var body struct {
		URL            string `json:"url"`
		Text           string `json:"text"`
		ImageURL       string `json:"image_url"`
		ImageBase64    string `json:"image_base64"`
		ImageMediaType string `json:"image_media_type"`
	}
	if err := readJSON(r, &body); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	ex := mcp.NewClaudeExtractor(s.cfg.AnthropicAPIKey)
	var draft *mcp.RecipeDraft
	var err error

	switch {
	case body.URL != "":
		text, fetchErr := fetchPageText(body.URL)
		if fetchErr != nil {
			errorJSON(w, http.StatusBadRequest, "impossible de charger la page : "+fetchErr.Error())
			return
		}
		draft, err = ex.ExtractFromText(r.Context(), "URL: "+body.URL+"\n\n"+text)
		if err == nil && draft.SourceURL == "" {
			draft.SourceURL = body.URL
		}

	case body.ImageURL != "":
		draft, err = ex.ExtractFromImageURL(r.Context(), body.ImageURL)

	case body.ImageBase64 != "":
		mt := body.ImageMediaType
		if mt == "" {
			mt = "image/jpeg"
		}
		draft, err = ex.ExtractFromImageBase64(r.Context(), mt, body.ImageBase64)

	case body.Text != "":
		draft, err = ex.ExtractFromText(r.Context(), body.Text)

	default:
		errorJSON(w, http.StatusBadRequest, "url, text, image_url ou image_base64 requis")
		return
	}

	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	recipe := &storage.Recipe{
		Title:        draft.Title,
		Description:  draft.Description,
		Instructions: draft.Instructions,
		Servings:     draft.Servings,
		Tags:         draft.Tags,
		SourceURL:    draft.SourceURL,
	}

	// Check for an existing recipe with the same URL or title → update it
	existing, err := s.db.FindByURLOrTitle(user.ID, draft.SourceURL, draft.Title)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	if existing != nil {
		// Preserve the existing photo if none comes with the import
		recipe.ImageURL = existing.ImageURL
		updated, err := s.db.UpdateRecipe(existing.ID, user.ID, recipe, draft.Ingredients)
		if err != nil {
			errorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	created, err := s.db.CreateRecipe(user.ID, recipe, draft.Ingredients)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// fetchPageText downloads a URL and returns its readable text content.
func fetchPageText(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RecipeBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "fr,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Limit to 512 KB to keep Claude input manageable
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}
	return stripHTML(string(body)), nil
}

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reTags        = regexp.MustCompile(`<[^>]+>`)
	reSpaces      = regexp.MustCompile(`\s+`)
)

func stripHTML(html string) string {
	html = reScriptStyle.ReplaceAllString(html, " ")
	html = reTags.ReplaceAllString(html, " ")
	html = reSpaces.ReplaceAllString(html, " ")
	return strings.TrimSpace(html)
}
