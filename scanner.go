package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Song represents a music file with its metadata
type Song struct {
	Path        string    // Absolute path to the file
	Title       string
	Artist      string
	Album       string
	ModTime     time.Time // File modification time
	displayName string    // Cached display name
	searchName  string    // Lowercase for searching
}

// Regex to parse filenames like "0016 - Enter Shikari - Sorry You're Not a Winner.flac"
// or "01 - Phony Rappers.flac"
var filenameRegex = regexp.MustCompile(`^(\d+)\s*-\s*(?:([^-]+)\s*-\s*)?(.+)$`)

// ParseMetadataFromPath extracts Artist, Album, Title from file path
// Path structure: Music/Artist/Album/NNNN - Artist - Title.ext
// or: Music/Artist/Album/NN - Title.ext
func ParseMetadataFromPath(path string) (artist, album, title string) {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	// Remove extension
	ext := filepath.Ext(filename)
	filename = strings.TrimSuffix(filename, ext)

	// Try to parse filename
	matches := filenameRegex.FindStringSubmatch(filename)
	if matches != nil {
		if matches[2] != "" {
			// Format: NNNN - Artist - Title
			artist = strings.TrimSpace(matches[2])
			title = strings.TrimSpace(matches[3])
		} else {
			// Format: NN - Title
			title = strings.TrimSpace(matches[3])
		}
	} else {
		// Fallback: use filename as title
		title = filename
	}

	// Get album from parent directory
	album = filepath.Base(dir)

	// If we didn't get artist from filename, get from grandparent directory
	if artist == "" {
		grandparent := filepath.Dir(dir)
		artist = filepath.Base(grandparent)
	}

	// Clean up "CD1", "CD2" etc from album names
	if strings.HasPrefix(strings.ToUpper(album), "CD") && len(album) <= 3 {
		// Album is just "CD1" etc, use parent
		album = filepath.Base(filepath.Dir(dir))
	}

	return artist, album, title
}

// DisplayName returns the formatted display string for the song browser
func (s Song) DisplayName() string {
	return s.displayName
}

// SearchName returns lowercase name for searching
func (s Song) SearchName() string {
	return s.searchName
}

// buildDisplayName creates the display string
func buildDisplayName(artist, title, album, path string) string {
	if artist == "" {
		artist = "Unknown Artist"
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if album == "" {
		album = "Unknown Album"
	}
	return artist + " - " + title + " (" + album + ")"
}

// ConfirmDisplayName returns the formatted display string for confirmation
func (s Song) ConfirmDisplayName() string {
	artist := s.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}
	title := s.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(s.Path), filepath.Ext(s.Path))
	}
	return artist + " - " + title
}

// RelativePath returns the path relative to the playlist directory
func (s Song) RelativePath(musicDir string) string {
	rel, err := filepath.Rel(filepath.Dir(musicDir), s.Path)
	if err != nil {
		return s.Path
	}
	return "../" + rel
}

// ScanMusicDirectory scans the music directory and returns all songs
// Metadata is parsed from file paths, not from file contents (fast)
func ScanMusicDirectory(musicDir string) ([]Song, error) {
	var songs []Song

	supportedExtensions := map[string]bool{
		".flac": true,
		".mp3":  true,
		".m4a":  true,
		".ogg":  true,
		".wav":  true,
	}

	err := filepath.Walk(musicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip hidden directories
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check if it's a supported audio file
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExtensions[ext] {
			return nil
		}

		// Parse metadata from path (no file I/O needed)
		artist, album, title := ParseMetadataFromPath(path)
		displayName := buildDisplayName(artist, title, album, path)

		songs = append(songs, Song{
			Path:        path,
			Artist:      artist,
			Album:       album,
			Title:       title,
			ModTime:     info.ModTime(),
			displayName: displayName,
			searchName:  strings.ToLower(displayName),
		})

		return nil
	})

	// Sort by modification time, newest first
	sort.Slice(songs, func(i, j int) bool {
		return songs[i].ModTime.After(songs[j].ModTime)
	})

	return songs, err
}
