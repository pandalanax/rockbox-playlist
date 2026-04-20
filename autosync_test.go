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
