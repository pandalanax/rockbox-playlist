package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Playlist represents a m3u8 playlist file
type Playlist struct {
	Name    string   // Display name (filename without extension)
	Path    string   // Full path to the playlist file
	Entries []string // List of relative paths in the playlist
}

// LoadPlaylists scans the playlist directory for m3u8 files
func LoadPlaylists(playlistDir string) ([]Playlist, error) {
	var playlists []Playlist

	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".m3u8" && ext != ".m3u" {
			continue
		}

		playlist := Playlist{
			Name: strings.TrimSuffix(name, ext),
			Path: filepath.Join(playlistDir, name),
		}

		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

// LoadPlaylistEntries reads all entries from a playlist file
func LoadPlaylistEntries(playlistPath string) ([]string, error) {
	var entries []string

	f, err := os.Open(playlistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil // Empty playlist
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}

	return entries, scanner.Err()
}

// AppendToPlaylist adds songs to the playlist file
func AppendToPlaylist(playlistPath string, entries []string) error {
	f, err := os.OpenFile(playlistPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range entries {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// NormalizePath normalizes a path for comparison (removes ../ prefix variations)
func NormalizePath(path string) string {
	// Remove leading ../ or ./ 
	path = strings.TrimPrefix(path, "../")
	path = strings.TrimPrefix(path, "./")
	return path
}
