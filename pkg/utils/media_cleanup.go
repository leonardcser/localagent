package utils

import (
	"os"
	"path/filepath"
	"time"

	"localagent/pkg/logger"
)

// CleanOldMedia removes files in mediaDir older than ttl based on modification time.
func CleanOldMedia(mediaDir string, ttl time.Duration) {
	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-ttl)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(mediaDir, entry.Name())
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				logger.Warn("media cleanup: failed to remove %s: %v", path, err)
			} else {
				removed++
			}
		}
	}

	if removed > 0 {
		logger.Info("media cleanup: removed %d expired file(s)", removed)
	}
}
