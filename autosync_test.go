package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestBuildRecentlyAddedEntries(t *testing.T) {
	files := []string{
		"Artist/Album/01 - Song.flac",
		"Artist/Album/cover.jpg",
		"Artist/Album/02 - Track.MP3",
		"Artist/Album/notes.txt",
	}

	got := BuildRecentlyAddedEntries(files, "Music")
	want := []string{
		"../Music/Artist/Album/01 - Song.flac",
		"../Music/Artist/Album/02 - Track.MP3",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildRecentlyAddedEntries() = %v, want %v", got, want)
	}
}

func TestUpdateRecentlyAddedFromSync(t *testing.T) {
	playlistDir := createPlaylistDir(t, map[string]string{
		"Recently Added.m3u8": "../Music/old.flac\n",
	})
	playlistPath := filepath.Join(playlistDir, "Recently Added.m3u8")

	count, err := UpdateRecentlyAddedFromSync(playlistPath, []string{
		"Artist/Album/01 - Song.flac",
		"Artist/Album/cover.jpg",
		"Artist/Album/02 - Track.mp3",
	}, "Music")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	got, err := LoadPlaylistEntries(playlistPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"../Music/old.flac",
		"../Music/Artist/Album/01 - Song.flac",
		"../Music/Artist/Album/02 - Track.mp3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("playlist entries = %v, want %v", got, want)
	}
}

func TestFindPodcastPlaylistPath(t *testing.T) {
	playlistDir := createPlaylistDir(t, map[string]string{
		"Rock.m3u8":     "",
		"Podcasts.m3u8": "",
	})

	got, err := FindPodcastPlaylistPath(playlistDir)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(playlistDir, "Podcasts.m3u8")
	if got != want {
		t.Fatalf("FindPodcastPlaylistPath() = %q, want %q", got, want)
	}
}

func TestWriteAutosyncStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status", "autosync.json")
	want := autosyncStatus{
		State:              "running",
		Phase:              "sync",
		Message:            "syncing",
		StartedAt:          time.Unix(123, 0).UTC(),
		SyncCount:          2,
		RecentlyAddedCount: 1,
		PodcastDownloaded:  3,
		PodcastDeleted:     1,
	}

	if err := writeAutosyncStatus(path, want); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var got autosyncStatus
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status = %+v, want %+v", got, want)
	}
}

func TestRunAutosyncSkipsWhenSessionActive(t *testing.T) {
	deviceRoot := createDeviceDir(t)
	sourceRoot := createMusicDir(t, "Artist/Album/01 - Song.flac")
	statusPath := filepath.Join(t.TempDir(), "status.json")

	ledRoot := t.TempDir()
	ledDir := filepath.Join(ledRoot, stateLEDName)
	if err := os.MkdirAll(ledDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trigger", "brightness", "delay_on", "delay_off"} {
		if err := os.WriteFile(filepath.Join(ledDir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	prevLEDBase := ledBasePath
	ledBasePath = ledRoot
	defer func() { ledBasePath = prevLEDBase }()

	_ = clearSessionActive()
	defer func() { _ = clearSessionActive() }()
	if err := markSessionActive(); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	summary, err := RunAutosync(cfg, deviceRoot, sourceRoot, statusPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.SyncCount != 0 || summary.PodcastDownloaded != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status autosyncStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != "skipped" || status.Phase != "skipped" {
		t.Fatalf("status = %+v, want skipped/skipped", status)
	}
	if status.Message != "interactive session active" {
		t.Fatalf("message = %q, want interactive session active", status.Message)
	}
}
