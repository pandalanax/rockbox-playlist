package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunSyncDryRun(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available")
	}

	src := createMusicDir(t,
		"Artist/Album/01 - Song.flac",
		"Artist/Album/02 - Track.mp3",
	)
	dst := t.TempDir()

	cmd := runSyncDryRun(src, dst)
	msg := cmd()

	result, ok := msg.(syncDryRunMsg)
	if !ok {
		t.Fatalf("unexpected message type: %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if len(result.files) != 2 {
		t.Errorf("got %d files, want 2: %v", len(result.files), result.files)
	}
}

func TestRunSync(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available")
	}

	// runSync uses --info=progress2 which macOS openrsync doesn't support.
	// Check if rsync supports it by running a quick test.
	check := exec.Command("rsync", "--info=progress2", "--version")
	if err := check.Run(); err != nil {
		t.Skip("rsync does not support --info=progress2 (likely macOS openrsync)")
	}

	src := createMusicDir(t,
		"Artist/Album/01 - Song.flac",
		"Artist/Album/02 - Track.mp3",
	)
	dst := t.TempDir()

	cmd := runSync(src, dst, 2)
	msg := cmd()

	result, ok := msg.(syncDoneMsg)
	if !ok {
		t.Fatalf("unexpected message type: %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
	}

	// Verify files were actually copied
	song1 := filepath.Join(dst, "Artist", "Album", "01 - Song.flac")
	if _, err := os.Stat(song1); err != nil {
		t.Errorf("expected file to exist at %q", song1)
	}
}
