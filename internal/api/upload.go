package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxUploadSize = 10 << 20 // 10 MB

var allowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		errorJSON(w, http.StatusBadRequest, "fichier trop volumineux (max 10 MB)")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "champ 'photo' manquant")
		return
	}
	defer file.Close()

	// Detect MIME type from first 512 bytes
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mime := http.DetectContentType(buf[:n])
	// Strip parameters (e.g. "image/jpeg; charset=...")
	mime = strings.Split(mime, ";")[0]
	mime = strings.TrimSpace(mime)

	ext, ok := allowedTypes[mime]
	if !ok {
		errorJSON(w, http.StatusBadRequest, "type de fichier non supporté (jpeg, png, webp, gif uniquement)")
		return
	}

	// Seek back to start after reading for MIME detection
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	} else {
		// Re-read from header if seek not supported
		_ = header
	}

	// Save to uploads/ directory
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		errorJSON(w, http.StatusInternalServerError, "impossible de créer le dossier uploads")
		return
	}

	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), uuid.NewString()[:8], ext)
	dst, err := os.Create(filepath.Join(uploadsDir, filename))
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "impossible de créer le fichier")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		errorJSON(w, http.StatusInternalServerError, "erreur lors de la sauvegarde")
		return
	}

	url := "/uploads/" + filename
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}
