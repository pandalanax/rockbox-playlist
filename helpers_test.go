package main

import (
	"os"
	"path/filepath"
	"testing"
)

// createMusicDir creates a temp directory with empty audio files at the given relative paths.
// Example: createMusicDir(t, "Artist1/Album1/01 - Song.flac", "Artist2/Album2/01 - Track.mp3")
func createMusicDir(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		path := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fake"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// createPlaylistDir creates a temp directory with .m3u8 files.
// Keys are filenames, values are file contents.
func createPlaylistDir(t *testing.T, playlists map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range playlists {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// createDeviceDir creates a fake Rockbox device with Music/, Playlists/, and Audiobooks/ dirs.
func createDeviceDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Music", "Playlists", "Audiobooks"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
