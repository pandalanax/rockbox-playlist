package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"../Music/foo.mp3", "Music/foo.mp3"},
		{"./Music/foo.mp3", "Music/foo.mp3"},
		{"Music/foo.mp3", "Music/foo.mp3"},
		{"foo.mp3", "foo.mp3"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizePath(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadPlaylists(t *testing.T) {
	dir := createPlaylistDir(t, map[string]string{
		"Rock.m3u8":    "../Music/song.flac\n",
		"Jazz.m3u":     "../Music/jazz.mp3\n",
		"notes.txt":    "not a playlist",
		"._Rock.m3u8":  "macOS resource fork",
	})
	// Also create a subdirectory that should be skipped
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	playlists, err := LoadPlaylists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 2 {
		t.Fatalf("got %d playlists, want 2", len(playlists))
	}

	// Check that names don't include extensions
	names := map[string]bool{}
	for _, p := range playlists {
		names[p.Name] = true
		if p.Path == "" {
			t.Error("playlist path is empty")
		}
	}
	if !names["Rock"] || !names["Jazz"] {
		t.Errorf("expected Rock and Jazz, got %v", names)
	}
}

func TestLoadPlaylists_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	playlists, err := LoadPlaylists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 0 {
		t.Fatalf("got %d playlists, want 0", len(playlists))
	}
}

func TestLoadPlaylists_NonExistentDir(t *testing.T) {
	_, err := LoadPlaylists("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestLoadPlaylistEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u8")
	content := "#EXTM3U\n\n# comment\n../Music/Artist/Album/song.flac\n\n../Music/Other/track.mp3\n"
	os.WriteFile(path, []byte(content), 0644)

	entries, err := LoadPlaylistEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0] != "../Music/Artist/Album/song.flac" {
		t.Errorf("got %q, want ../Music/Artist/Album/song.flac", entries[0])
	}
}

func TestLoadPlaylistEntries_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u8")
	content := "../Music/Guns N' Roses/Appetite/01 - Welcome to the Jungle.flac\n" +
		"../Music/Björk/Homogenic/03 - Jóga.flac\n" +
		"../Music/坂本龍一/千のナイフ/01 - 千のナイフ.flac\n" +
		"../Music/AC⧸DC/Back in Black/01 - Hells Bells.mp3\n" +
		"../Music/Simon & Garfunkel/Hits/01 - The Sound of Silence.mp3\n"
	os.WriteFile(path, []byte(content), 0644)

	entries, err := LoadPlaylistEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}
	// Verify special chars are preserved exactly
	if entries[0] != "../Music/Guns N' Roses/Appetite/01 - Welcome to the Jungle.flac" {
		t.Errorf("apostrophe path not preserved: %q", entries[0])
	}
	if entries[2] != "../Music/坂本龍一/千のナイフ/01 - 千のナイフ.flac" {
		t.Errorf("japanese path not preserved: %q", entries[2])
	}
}

func TestAppendToPlaylist_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u8")

	entries := []string{
		"../Music/Motörhead/Ace of Spades/01 - Ace of Spades.mp3",
		"../Music/Sigur Rós/( )/01 - Untitled.flac",
		"../Music/Godspeed You! Black Emperor/F♯A♯∞/01 - The Dead Flag Blues.mp3",
	}
	err := AppendToPlaylist(path, entries)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 3 {
		t.Fatalf("got %d entries, want 3", len(loaded))
	}
	for i, want := range entries {
		if loaded[i] != want {
			t.Errorf("entry %d: got %q, want %q", i, loaded[i], want)
		}
	}
}

func TestNormalizePath_SpecialCharacters(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"../Music/Björk/Jóga.flac", "Music/Björk/Jóga.flac"},
		{"../Music/Guns N' Roses/Song.mp3", "Music/Guns N' Roses/Song.mp3"},
		{"./Music/坂本龍一/Track.flac", "Music/坂本龍一/Track.flac"},
		{"../Music/AC⧸DC/Song.mp3", "Music/AC⧸DC/Song.mp3"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizePath(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadPlaylists_SpecialCharacterNames(t *testing.T) {
	dir := createPlaylistDir(t, map[string]string{
		"Rock & Roll.m3u8":       "../Music/song.flac\n",
		"Björk's Favorites.m3u8": "../Music/joga.flac\n",
		"日本語.m3u8":               "../Music/track.flac\n",
	})

	playlists, err := LoadPlaylists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 3 {
		t.Fatalf("got %d playlists, want 3", len(playlists))
	}

	names := map[string]bool{}
	for _, p := range playlists {
		names[p.Name] = true
	}
	for _, want := range []string{"Rock & Roll", "Björk's Favorites", "日本語"} {
		if !names[want] {
			t.Errorf("missing playlist %q in %v", want, names)
		}
	}
}

func TestLoadPlaylistEntries_NonExistent(t *testing.T) {
	entries, err := LoadPlaylistEntries("/nonexistent/file.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestUpdateRecentlyAdded_NewPlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	entries := []string{"../Music/Artist/song1.flac", "../Music/Artist/song2.flac"}
	err := UpdateRecentlyAdded(path, entries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPlaylistEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("got %d entries, want 2", len(loaded))
	}
	if loaded[0] != entries[0] || loaded[1] != entries[1] {
		t.Errorf("got %v, want %v", loaded, entries)
	}
}

func TestUpdateRecentlyAdded_AppendUnderLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	// Write 10 existing entries
	var existing []string
	for i := 0; i < 10; i++ {
		existing = append(existing, fmt.Sprintf("../Music/old/track%02d.flac", i))
	}
	os.WriteFile(path, []byte(strings.Join(existing, "\n")+"\n"), 0644)

	// Append 5 new
	var newEntries []string
	for i := 0; i < 5; i++ {
		newEntries = append(newEntries, fmt.Sprintf("../Music/new/track%02d.flac", i))
	}
	err := UpdateRecentlyAdded(path, newEntries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 15 {
		t.Fatalf("got %d entries, want 15", len(loaded))
	}
}

func TestUpdateRecentlyAdded_TrimOldest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	// Write 95 existing entries
	var existing []string
	for i := 0; i < 95; i++ {
		existing = append(existing, fmt.Sprintf("../Music/old/track%03d.flac", i))
	}
	os.WriteFile(path, []byte(strings.Join(existing, "\n")+"\n"), 0644)

	// Append 10 new → 105 total, should trim to 100
	var newEntries []string
	for i := 0; i < 10; i++ {
		newEntries = append(newEntries, fmt.Sprintf("../Music/new/track%02d.flac", i))
	}
	err := UpdateRecentlyAdded(path, newEntries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 100 {
		t.Fatalf("got %d entries, want 100", len(loaded))
	}
	// Oldest 5 should have been trimmed (old/track000 through old/track004)
	if loaded[0] != "../Music/old/track005.flac" {
		t.Errorf("first entry should be track005, got %q", loaded[0])
	}
	// Last entry should be the last new one
	if loaded[99] != "../Music/new/track09.flac" {
		t.Errorf("last entry should be new/track09, got %q", loaded[99])
	}
}

func TestUpdateRecentlyAdded_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	entries := []string{
		"../Music/Björk/Homogenic/03 - Jóga.flac",
		"../Music/坂本龍一/千のナイフ/01 - 千のナイフ.flac",
		"../Music/Guns N' Roses/Appetite/01 - Welcome to the Jungle.flac",
	}
	err := UpdateRecentlyAdded(path, entries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 3 {
		t.Fatalf("got %d entries, want 3", len(loaded))
	}
	for i, want := range entries {
		if loaded[i] != want {
			t.Errorf("entry %d: got %q, want %q", i, loaded[i], want)
		}
	}
}

func TestUpdateRecentlyAdded_ExactlyAtLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	// Write exactly 100 existing entries
	var existing []string
	for i := 0; i < 100; i++ {
		existing = append(existing, fmt.Sprintf("../Music/old/track%03d.flac", i))
	}
	os.WriteFile(path, []byte(strings.Join(existing, "\n")+"\n"), 0644)

	// Append 1 new entry → 101 total, should trim to 100
	err := UpdateRecentlyAdded(path, []string{"../Music/new/latest.flac"}, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 100 {
		t.Fatalf("got %d entries, want 100", len(loaded))
	}
	// Oldest entry (track000) should be gone
	if loaded[0] != "../Music/old/track001.flac" {
		t.Errorf("first entry should be track001, got %q", loaded[0])
	}
	// Last entry should be the new one
	if loaded[99] != "../Music/new/latest.flac" {
		t.Errorf("last entry should be latest.flac, got %q", loaded[99])
	}
}

func TestUpdateRecentlyAdded_BulkOverflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	// Write 50 existing entries
	var existing []string
	for i := 0; i < 50; i++ {
		existing = append(existing, fmt.Sprintf("../Music/old/track%03d.flac", i))
	}
	os.WriteFile(path, []byte(strings.Join(existing, "\n")+"\n"), 0644)

	// Append 120 new entries → 170 total, should trim to 100, all old gone
	var newEntries []string
	for i := 0; i < 120; i++ {
		newEntries = append(newEntries, fmt.Sprintf("../Music/new/track%03d.flac", i))
	}
	err := UpdateRecentlyAdded(path, newEntries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 100 {
		t.Fatalf("got %d entries, want 100", len(loaded))
	}
	// First entry should be new/track020 (120-100=20 trimmed from new entries)
	if loaded[0] != "../Music/new/track020.flac" {
		t.Errorf("first entry should be new/track020, got %q", loaded[0])
	}
	if loaded[99] != "../Music/new/track119.flac" {
		t.Errorf("last entry should be new/track119, got %q", loaded[99])
	}
}

func TestUpdateRecentlyAdded_EmojiPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Recently Added.m3u8")

	entries := []string{
		"../Music/🎵 Party Mix/01 - 🔥 Fire.mp3",
		"../Music/Artist/Album/🌸 Cherry Blossoms.flac",
		"../Music/DJ 💿/Set/01 - Drop 🎤.m4a",
	}
	err := UpdateRecentlyAdded(path, entries, 100)
	if err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPlaylistEntries(path)
	if len(loaded) != 3 {
		t.Fatalf("got %d entries, want 3", len(loaded))
	}
	for i, want := range entries {
		if loaded[i] != want {
			t.Errorf("entry %d: got %q, want %q", i, loaded[i], want)
		}
	}
}

func TestAppendToPlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u8")

	// Append to new file
	err := AppendToPlaylist(path, []string{"../Music/song1.flac", "../Music/song2.mp3"})
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := LoadPlaylistEntries(path)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// Append more
	err = AppendToPlaylist(path, []string{"../Music/song3.ogg"})
	if err != nil {
		t.Fatal(err)
	}

	entries, _ = LoadPlaylistEntries(path)
	if len(entries) != 3 {
		t.Fatalf("got %d entries after second append, want 3", len(entries))
	}
}
