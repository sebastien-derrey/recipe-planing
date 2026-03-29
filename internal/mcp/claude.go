package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"recipe_manager/internal/storage"
)

// RecipeDraft is the structured output Claude extracts from content.
type RecipeDraft struct {
	Title        string                    `json:"title"`
	Description  string                    `json:"description"`
	Instructions string                    `json:"instructions"`
	Servings     int                       `json:"servings"`
	Tags         []string                  `json:"tags,omitempty"`
	Ingredients  []storage.IngredientInput `json:"ingredients"`
	SourceURL    string                    `json:"source_url,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

const extractionPrompt = `Extract the recipe from the provided content and return ONLY valid JSON with no markdown, no extra text:
{
  "title": "string",
  "description": "short one-line summary",
  "instructions": "full cooking instructions as plain text",
  "servings": integer (number of people, default 4),
  "tags": ["tag1", "tag2"],
  "ingredients": [
    { "name": "ingredient name in lowercase", "quantity": number, "unit": "g|ml|cs|cc|tasse|pièce|botte|gousse|pincée|etc", "notes": "optional preparation notes" }
  ]
}
If no recipe is found in the content, return: {"error": "no recipe found"}`

// ClaudeExtractor uses the Anthropic API to extract recipes from text or images.
type ClaudeExtractor struct {
	client anthropic.Client
}

// NewClaudeExtractor creates a ClaudeExtractor with the given API key.
func NewClaudeExtractor(apiKey string) *ClaudeExtractor {
	return &ClaudeExtractor{client: anthropic.NewClient(option.WithAPIKey(apiKey))}
}

// ExtractFromText asks Claude to extract a recipe from plain text.
func (e *ClaudeExtractor) ExtractFromText(ctx context.Context, text string) (*RecipeDraft, error) {
	msg, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(extractionPrompt + "\n\nContent:\n" + text),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}
	return parseDraft(msg)
}

// ExtractFromImageBase64 asks Claude to extract a recipe from a base64-encoded image.
func (e *ClaudeExtractor) ExtractFromImageBase64(ctx context.Context, mediaType, base64Data string) (*RecipeDraft, error) {
	msg, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(extractionPrompt),
				anthropic.NewImageBlockBase64(mediaType, base64Data),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}
	return parseDraft(msg)
}

// ExtractFromImageURL asks Claude to extract a recipe from an image.
func (e *ClaudeExtractor) ExtractFromImageURL(ctx context.Context, imageURL string) (*RecipeDraft, error) {
	msg, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(extractionPrompt),
				anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: imageURL}),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}
	return parseDraft(msg)
}

func parseDraft(msg *anthropic.Message) (*RecipeDraft, error) {
	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}
	raw := ""
	for _, block := range msg.Content {
		if block.Type == "text" {
			raw = block.Text
			break
		}
	}
	// Strip any accidental markdown fences
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var draft RecipeDraft
	if err := json.Unmarshal([]byte(raw), &draft); err != nil {
		return nil, fmt.Errorf("parsing Claude response: %w (raw: %s)", err, raw)
	}
	if draft.Error != "" {
		return nil, fmt.Errorf("no recipe found: %s", draft.Error)
	}
	if draft.Servings == 0 {
		draft.Servings = 4
	}
	return &draft, nil
}
