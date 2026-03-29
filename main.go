package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"recipe_manager/internal/api"
	"recipe_manager/internal/config"
	"recipe_manager/internal/mcp"
	"recipe_manager/internal/storage"
)

//go:embed web
var webFS embed.FS

func main() {
	mcpMode := flag.Bool("mcp", false, "Run as MCP stdio server")
	flag.Parse()

	if *mcpMode {
		runMCP()
		return
	}
	runServer()
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Config warning: %v (using defaults)", err)
	}

	db, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	// API + auth routes
	api.New(mux, db, cfg)

	// Serve embedded web files
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web embed: %v", err)
	}
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webSub))))

	// Serve uploaded photos from disk (not embedded — written at runtime)
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatalf("uploads dir: %v", err)
	}
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// SPA fallback — serve index.html for all other GET requests
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path == "/" {
			data, _ := webFS.ReadFile("web/index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		// Let non-root paths fall through to 404 unless matched above
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("recipe_manager listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runMCP() {
	apiBase  := os.Getenv("RECIPE_MANAGER_API")
	token    := os.Getenv("RECIPE_MANAGER_MCP_TOKEN")
	apiKey   := os.Getenv("ANTHROPIC_API_KEY")

	if apiBase == "" {
		apiBase = "http://localhost:8080"
	}
	if apiKey == "" {
		// Try to load from config.json
		if cfg, err := config.Load(); err == nil {
			apiKey = cfg.AnthropicAPIKey
			if token == "" {
				token = cfg.MCPServiceToken
			}
		}
	}
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY env var (or anthropic_api_key in config.json) is required for MCP mode")
	}

	log.SetOutput(os.Stderr) // MCP uses stdout for JSON-RPC
	log.Printf("MCP server starting, api=%s", apiBase)
	mcp.Run(apiBase, token, apiKey)
}
