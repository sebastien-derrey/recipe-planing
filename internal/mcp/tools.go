package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"recipe_manager/internal/storage"
)

// toolDef describes an MCP tool for the tools/list response.
type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

var tools = []toolDef{
	{
		Name:        "add_recipe_from_text",
		Description: "Extract and save a recipe from plain text (e.g. copied from a web page or book).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text":    map[string]any{"type": "string", "description": "The raw text containing the recipe"},
				"user_id": map[string]any{"type": "string", "description": "The ID of the user to save the recipe for"},
			},
			"required": []string{"text", "user_id"},
		},
	},
	{
		Name:        "add_recipe_from_image_url",
		Description: "Extract and save a recipe from a publicly accessible image URL (e.g. photo of a cookbook page).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_url": map[string]any{"type": "string", "description": "Public URL of the image"},
				"user_id":   map[string]any{"type": "string", "description": "The ID of the user to save the recipe for"},
			},
			"required": []string{"image_url", "user_id"},
		},
	},
	{
		Name:        "add_recipe_from_video",
		Description: "Extract and save a recipe from a YouTube video URL using its title and description.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"video_url": map[string]any{"type": "string", "description": "YouTube video URL"},
				"user_id":   map[string]any{"type": "string", "description": "The ID of the user to save the recipe for"},
			},
			"required": []string{"video_url", "user_id"},
		},
	},
	{
		Name:        "list_recipes",
		Description: "List all recipes saved by a user.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"user_id": map[string]any{"type": "string"}},
			"required":   []string{"user_id"},
		},
	},
	{
		Name:        "get_weekly_plan",
		Description: "Get the weekly meal plan and shopping list for a given week.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string"},
				"week":    map[string]any{"type": "string", "description": "Monday date of the week (YYYY-MM-DD)"},
			},
			"required": []string{"user_id", "week"},
		},
	},
}

// dispatcher routes tool calls to their implementations.
type dispatcher struct {
	extractor  *ClaudeExtractor
	apiBase    string
	mcpToken   string
	httpClient *http.Client
}

func newDispatcher(extractor *ClaudeExtractor, apiBase, mcpToken string) *dispatcher {
	return &dispatcher{
		extractor:  extractor,
		apiBase:    apiBase,
		mcpToken:   mcpToken,
		httpClient: &http.Client{},
	}
}

func (d *dispatcher) dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "add_recipe_from_text":
		return d.addRecipeFromText(ctx, args)
	case "add_recipe_from_image_url":
		return d.addRecipeFromImageURL(ctx, args)
	case "add_recipe_from_video":
		return d.addRecipeFromVideo(ctx, args)
	case "list_recipes":
		return d.listRecipes(ctx, args)
	case "get_weekly_plan":
		return d.getWeeklyPlan(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (d *dispatcher) addRecipeFromText(ctx context.Context, args map[string]any) (string, error) {
	text, _ := args["text"].(string)
	userID, _ := args["user_id"].(string)
	if text == "" || userID == "" {
		return "", fmt.Errorf("text and user_id are required")
	}
	draft, err := d.extractor.ExtractFromText(ctx, text)
	if err != nil {
		return "", err
	}
	return d.saveRecipe(ctx, userID, draft, "")
}

func (d *dispatcher) addRecipeFromImageURL(ctx context.Context, args map[string]any) (string, error) {
	imageURL, _ := args["image_url"].(string)
	userID, _ := args["user_id"].(string)
	if imageURL == "" || userID == "" {
		return "", fmt.Errorf("image_url and user_id are required")
	}
	draft, err := d.extractor.ExtractFromImageURL(ctx, imageURL)
	if err != nil {
		return "", err
	}
	return d.saveRecipe(ctx, userID, draft, imageURL)
}

func (d *dispatcher) addRecipeFromVideo(ctx context.Context, args map[string]any) (string, error) {
	videoURL, _ := args["video_url"].(string)
	userID, _ := args["user_id"].(string)
	if videoURL == "" || userID == "" {
		return "", fmt.Errorf("video_url and user_id are required")
	}

	// Fetch video metadata via YouTube oEmbed (no API key needed)
	oembedURL := "https://www.youtube.com/oembed?url=" + url.QueryEscape(videoURL) + "&format=json"
	resp, err := d.httpClient.Get(oembedURL)
	if err != nil {
		return "", fmt.Errorf("fetching video info: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var oembed struct {
		Title       string `json:"title"`
		AuthorName  string `json:"author_name"`
	}
	_ = json.Unmarshal(body, &oembed)

	text := fmt.Sprintf("Video title: %s\nChannel: %s\nVideo URL: %s\n\nPlease extract the recipe if this looks like a cooking video.",
		oembed.Title, oembed.AuthorName, videoURL)

	draft, err := d.extractor.ExtractFromText(ctx, text)
	if err != nil {
		return "", err
	}
	draft.SourceURL = videoURL
	return d.saveRecipe(ctx, userID, draft, "")
}

func (d *dispatcher) listRecipes(ctx context.Context, args map[string]any) (string, error) {
	userID, _ := args["user_id"].(string)
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET",
		d.apiBase+"/api/mcp/recipes?user_id="+url.QueryEscape(userID), nil)
	req.Header.Set("X-MCP-Token", d.mcpToken)
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (d *dispatcher) getWeeklyPlan(ctx context.Context, args map[string]any) (string, error) {
	userID, _ := args["user_id"].(string)
	week, _ := args["week"].(string)
	if userID == "" || week == "" {
		return "", fmt.Errorf("user_id and week are required")
	}
	endpoint := fmt.Sprintf("%s/api/mcp/weekly-plan?user_id=%s&week=%s",
		d.apiBase, url.QueryEscape(userID), url.QueryEscape(week))
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	req.Header.Set("X-MCP-Token", d.mcpToken)
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func (d *dispatcher) saveRecipe(ctx context.Context, userID string, draft *RecipeDraft, sourceURL string) (string, error) {
	if draft.SourceURL == "" && sourceURL != "" {
		draft.SourceURL = sourceURL
	}
	payload := map[string]any{
		"title":        draft.Title,
		"description":  draft.Description,
		"instructions": draft.Instructions,
		"servings":     draft.Servings,
		"source_url":   draft.SourceURL,
		"ingredients":  draft.Ingredients,
	}
	// Convert ingredients to the right type
	ingrs := make([]storage.IngredientInput, 0, len(draft.Ingredients))
	for _, i := range draft.Ingredients {
		ingrs = append(ingrs, storage.IngredientInput{
			Name:     i.Name,
			Quantity: i.Quantity,
			Unit:     i.Unit,
			Notes:    i.Notes,
		})
	}
	payload["ingredients"] = ingrs

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		d.apiBase+"/api/mcp/recipes?user_id="+url.QueryEscape(userID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MCP-Token", d.mcpToken)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("saving recipe: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}
	return fmt.Sprintf(`{"status":"created","recipe":%s}`, string(respBody)), nil
}
