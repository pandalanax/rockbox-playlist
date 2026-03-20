package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	content := "hello world"
	os.WriteFile(src, []byte(content), 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("copied content = %q, want %q", string(got), content)
	}
}

func TestPruneOldBackups(t *testing.T) {
	dir := t.TempDir()

	// Create 5 timestamped backup dirs + 1 .tmp dir
	backups := []string{
		"2024-01-01_10-00-00",
		"2024-01-02_10-00-00",
		"2024-01-03_10-00-00",
		"2024-01-04_10-00-00",
		"2024-01-05_10-00-00",
	}
	for _, b := range backups {
		os.MkdirAll(filepath.Join(dir, b), 0755)
	}
	os.MkdirAll(filepath.Join(dir, ".tmp-inprogress"), 0755)

	pruneOldBackups(dir, 3)

	entries, _ := os.ReadDir(dir)
	var remaining []string
	for _, e := range entries {
		remaining = append(remaining, e.Name())
	}

	// Should keep 3 newest + .tmp dir
	if len(remaining) != 4 {
		t.Fatalf("got %d entries, want 4 (3 backups + .tmp): %v", len(remaining), remaining)
	}

	// Oldest 2 should be gone
	for _, old := range []string{"2024-01-01_10-00-00", "2024-01-02_10-00-00"} {
		if _, err := os.Stat(filepath.Join(dir, old)); err == nil {
			t.Errorf("old backup %q should have been pruned", old)
		}
	}
}

func TestPruneOldBackups_UnderLimit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "2024-01-01_10-00-00"), 0755)
	os.MkdirAll(filepath.Join(dir, "2024-01-02_10-00-00"), 0755)

	pruneOldBackups(dir, 5)

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("should not prune when under limit, got %d entries", len(entries))
	}
}

func TestBackupPlaylists(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create playlist dir with files
	playlistDir := createPlaylistDir(t, map[string]string{
		"Rock.m3u8": "../Music/song.flac\n",
		"Jazz.m3u8": "../Music/jazz.mp3\n",
	})

	// Create podcast config
	podcastPath := filepath.Join(t.TempDir(), "podcasts.json")
	os.WriteFile(podcastPath, []byte(`{"Test":{"feedUrl":"http://example.com"}}`), 0644)

	summary, err := BackupPlaylists(playlistDir, podcastPath, 10)
	if err != nil {
		t.Fatal(err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Verify backup dir exists
	backupDir := filepath.Join(fakeHome, ".rockbox-playlist", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d backup dirs, want 1", len(entries))
	}

	// Verify files were copied
	backupFiles, _ := os.ReadDir(filepath.Join(backupDir, entries[0].Name()))
	if len(backupFiles) != 3 { // 2 playlists + podcasts.json
		t.Errorf("got %d backed up files, want 3", len(backupFiles))
	}
}

func TestBackupPlaylists_NoPlaylists(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	emptyDir := t.TempDir()
	_, err := BackupPlaylists(emptyDir, "/nonexistent/podcasts.json", 10)
	if err == nil {
		t.Error("expected error for empty playlist dir")
	}
}
