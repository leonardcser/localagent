package utils

import (
	"path/filepath"
	"strings"
)

// IsImageFile checks if a file is an image based on its filename extension.
func IsImageFile(filename string) bool {
	imageExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, ie := range imageExtensions {
		if ext == ie {
			return true
		}
	}
	return false
}

// DetectMIMEType returns the MIME type for a file based on its extension.
func DetectMIMEType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// SanitizeFilename removes potentially dangerous characters from a filename
// and returns a safe version for local filesystem storage.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, "..", "")
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	return base
}

