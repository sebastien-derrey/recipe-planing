package main

import (
	"embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
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

	// PWA files served at root scope (service worker scope must match app root)
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := webFS.ReadFile("web/sw.js")
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Write(data)
	})
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, _ := webFS.ReadFile("web/manifest.json")
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Write(data)
	})
	mux.HandleFunc("GET /icon-192.png", makeIcon(192))
	mux.HandleFunc("GET /icon-512.png", makeIcon(512))

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

// makeIcon returns a handler that generates a solid-green PNG icon of the given size.
func makeIcon(size int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0x2d, 0x6a, 0x4f, 0xff}}, image.Point{}, draw.Src)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		png.Encode(w, img)
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
