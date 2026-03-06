package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxBackups = 10

// getBackupDir returns the backup directory path (~/.rockbox-playlist/backups/)
func getBackupDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".rockbox-playlist", "backups"), nil
}

// BackupPlaylists creates a transactional backup of all playlists and podcast config
func BackupPlaylists(playlistDir, podcastConfigPath string) (string, error) {
	backupDir, err := getBackupDir()
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	tmpDir := filepath.Join(backupDir, ".tmp-"+timestamp)
	finalDir := filepath.Join(backupDir, timestamp)

	// Create temp directory
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("could not create backup directory: %w", err)
	}

	// On failure, clean up temp directory
	success := false
	defer func() {
		if !success {
			os.RemoveAll(tmpDir)
		}
	}()

	// Copy all playlist files
	fileCount := 0
	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return "", fmt.Errorf("could not read playlist directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".m3u" && ext != ".m3u8" {
			continue
		}

		src := filepath.Join(playlistDir, entry.Name())
		dst := filepath.Join(tmpDir, entry.Name())
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("could not backup %s: %w", entry.Name(), err)
		}
		fileCount++
	}

	// Copy podcast config if it exists
	if _, err := os.Stat(podcastConfigPath); err == nil {
		dst := filepath.Join(tmpDir, "podcasts.json")
		if err := copyFile(podcastConfigPath, dst); err != nil {
			return "", fmt.Errorf("could not backup podcasts.json: %w", err)
		}
		fileCount++
	}

	if fileCount == 0 {
		return "", fmt.Errorf("no playlists found to backup")
	}

	// Atomic rename: temp dir -> final dir
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", fmt.Errorf("could not finalize backup: %w", err)
	}
	success = true

	// Prune old backups
	pruneOldBackups(backupDir, maxBackups)

	summary := fmt.Sprintf("Backed up %d files to ~/%s",
		fileCount,
		filepath.Join(".rockbox-playlist", "backups", timestamp))
	return summary, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

// pruneOldBackups removes the oldest backups beyond the keep limit
func pruneOldBackups(backupDir string, keep int) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	// Collect valid backup directories (skip .tmp-* and non-directories)
	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			continue
		}
		backups = append(backups, entry.Name())
	}

	if len(backups) <= keep {
		return
	}

	// Sort by name ascending (timestamps sort naturally)
	sort.Strings(backups)

	// Remove oldest entries
	for _, name := range backups[:len(backups)-keep] {
		os.RemoveAll(filepath.Join(backupDir, name))
	}
}
