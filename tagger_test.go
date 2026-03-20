package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	id3v2 "github.com/bogem/id3v2/v2"
)

// minimalMP3 returns a minimal valid MP3 frame (MPEG1 Layer3 128kbps 44100Hz stereo).
// This is enough for id3v2 to open, tag, and save.
func minimalMP3() []byte {
	// MP3 sync word + valid frame header: 0xFF 0xFB 0x90 0x00
	// followed by enough zero bytes to fill the frame (417 bytes for this config)
	frame := make([]byte, 417)
	frame[0] = 0xFF
	frame[1] = 0xFB
	frame[2] = 0x90
	frame[3] = 0x00
	return frame
}

func TestTagMP3(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "episode.mp3")
	os.WriteFile(mp3Path, minimalMP3(), 0644)

	coverPath := filepath.Join(dir, "cover.jpg")
	coverData := []byte("fake-jpeg-data")
	os.WriteFile(coverPath, coverData, 0644)

	err := TagEpisode(mp3Path, "My Podcast", "Episode 1", coverPath)
	if err != nil {
		t.Fatalf("TagEpisode failed: %v", err)
	}

	// Re-read and verify tags
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("could not re-open tagged MP3: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "Episode 1" {
		t.Errorf("title = %q, want %q", tag.Title(), "Episode 1")
	}
	if tag.Artist() != "My Podcast" {
		t.Errorf("artist = %q, want %q", tag.Artist(), "My Podcast")
	}
	if tag.Album() != "My Podcast" {
		t.Errorf("album = %q, want %q", tag.Album(), "My Podcast")
	}

	// Check cover art
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pics) == 0 {
		t.Fatal("no picture frames found")
	}
	pic, ok := pics[0].(id3v2.PictureFrame)
	if !ok {
		t.Fatal("could not cast to PictureFrame")
	}
	if !bytes.Equal(pic.Picture, coverData) {
		t.Errorf("cover art data mismatch: got %d bytes, want %d", len(pic.Picture), len(coverData))
	}
}

func TestTagMP3_NoCover(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "episode.mp3")
	os.WriteFile(mp3Path, minimalMP3(), 0644)

	err := TagEpisode(mp3Path, "My Podcast", "Episode 1", "")
	if err != nil {
		t.Fatalf("TagEpisode without cover failed: %v", err)
	}

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("could not re-open: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "Episode 1" {
		t.Errorf("title = %q, want %q", tag.Title(), "Episode 1")
	}
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pics) != 0 {
		t.Errorf("expected no picture frames, got %d", len(pics))
	}
}

func TestTagEpisode_NonMP3(t *testing.T) {
	dir := t.TempDir()
	m4aPath := filepath.Join(dir, "episode.m4a")
	os.WriteFile(m4aPath, []byte("fake-m4a"), 0644)

	err := TagEpisode(m4aPath, "Podcast", "Episode", "")
	if err != nil {
		t.Errorf("expected nil for non-MP3, got: %v", err)
	}
}

func TestEnsureCoverArt_EmptyURL(t *testing.T) {
	dir := t.TempDir()
	path, err := EnsureCoverArt(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestEnsureCoverArt_AlreadyCached(t *testing.T) {
	dir := t.TempDir()
	coverPath := filepath.Join(dir, "cover.jpg")
	os.WriteFile(coverPath, []byte("existing-art"), 0644)

	path, err := EnsureCoverArt(dir, "https://should-not-be-fetched.example.com/art.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != coverPath {
		t.Errorf("path = %q, want %q", path, coverPath)
	}
}

func TestEnsureCoverArt_Downloads(t *testing.T) {
	artData := []byte("fake-jpeg-image-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artData)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path, err := EnsureCoverArt(dir, srv.URL+"/art.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != filepath.Join(dir, "cover.jpg") {
		t.Errorf("path = %q, want cover.jpg in dir", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read cover: %v", err)
	}
	if !bytes.Equal(data, artData) {
		t.Errorf("cover data mismatch")
	}
}

func TestFetchCoverArt(t *testing.T) {
	artData := []byte("test-cover-art-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(artData)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "cover.jpg")

	err := FetchCoverArt(srv.URL+"/art.jpg", dest)
	if err != nil {
		t.Fatalf("FetchCoverArt failed: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, artData) {
		t.Errorf("data mismatch")
	}
}

// TestTagMP3_EndToEnd fetches real cover art, tags a real MP3, and verifies the result.
func TestTagMP3_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	artworkURL := "https://is1-ssl.mzstatic.com/image/thumb/Podcasts211/v4/4c/df/da/4cdfda29-8f01-61e2-f183-1ca7791adbad/mza_14708591156495163514.jpeg/100x100bb.jpg"

	dir := t.TempDir()
	podcastDir := filepath.Join(dir, "DRINNIES")
	os.MkdirAll(podcastDir, 0755)

	// Create MP3
	mp3Path := filepath.Join(podcastDir, "DRINNIES-Test Episode.mp3")
	os.WriteFile(mp3Path, minimalMP3(), 0644)

	// EnsureCoverArt should download cover.jpg
	coverPath, err := EnsureCoverArt(podcastDir, artworkURL)
	t.Logf("EnsureCoverArt: path=%q err=%v", coverPath, err)
	if err != nil {
		t.Fatalf("EnsureCoverArt failed: %v", err)
	}
	if coverPath == "" {
		t.Fatal("coverPath is empty")
	}

	// Verify cover.jpg exists and has content
	info, err := os.Stat(coverPath)
	if err != nil {
		t.Fatalf("cover.jpg not created: %v", err)
	}
	t.Logf("cover.jpg size: %d bytes", info.Size())
	if info.Size() == 0 {
		t.Fatal("cover.jpg is empty")
	}

	// Tag the episode
	err = TagEpisode(mp3Path, "DRINNIES", "Test Episode", coverPath)
	t.Logf("TagEpisode: err=%v", err)
	if err != nil {
		t.Fatalf("TagEpisode failed: %v", err)
	}

	// Re-read and verify
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("could not re-open: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "Test Episode" {
		t.Errorf("title = %q", tag.Title())
	}
	if tag.Artist() != "DRINNIES" {
		t.Errorf("artist = %q", tag.Artist())
	}

	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	t.Logf("picture frames: %d", len(pics))
	if len(pics) == 0 {
		t.Error("no cover art embedded")
	} else {
		pic := pics[0].(id3v2.PictureFrame)
		t.Logf("embedded picture: %d bytes, mime=%s", len(pic.Picture), pic.MimeType)
		if len(pic.Picture) == 0 {
			t.Error("embedded picture is empty")
		}
	}
}

// TestAddPodcastWithLog_TagsEpisodes tests the full TUI flow: search → add → verify tags
func TestAddPodcastWithLog_TagsEpisodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	// Search iTunes for DRINNIES (same as TUI does)
	results, err := SearchPodcasts("DRINNIES")
	if err != nil {
		t.Fatalf("SearchPodcasts: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results from iTunes")
	}
	podcast := results[0]
	t.Logf("iTunes result: name=%q artworkURL=%q feedURL=%q", podcast.CollectionName, podcast.ArtworkURL, podcast.FeedURL)

	if podcast.ArtworkURL == "" {
		t.Fatal("iTunes returned empty artworkUrl100")
	}

	// Set up temp audioDir
	dir := t.TempDir()
	config := make(PodcastConfig)

	log, err := AddPodcastWithLog(podcast, dir, config, 1) // just 1 episode to be fast
	t.Logf("AddPodcastWithLog log:\n%s", joinLines(log))
	if err != nil {
		t.Fatalf("AddPodcastWithLog: %v", err)
	}

	// Check that cover.jpg was created
	folderName := SanitizeName(podcast.CollectionName)
	podcastDir := filepath.Join(dir, folderName)
	coverPath := filepath.Join(podcastDir, "cover.jpg")

	info, statErr := os.Stat(coverPath)
	if statErr != nil {
		t.Fatalf("cover.jpg not created: %v", statErr)
	}
	t.Logf("cover.jpg: %d bytes", info.Size())

	// Check that the episode MP3 has ID3 tags
	feed := config[podcast.CollectionName]
	if len(feed.Episodes) == 0 {
		t.Fatal("no episodes in config")
	}
	epPath := filepath.Join(podcastDir, feed.Episodes[0].File)
	tag, tagErr := id3v2.Open(epPath, id3v2.Options{Parse: true})
	if tagErr != nil {
		t.Fatalf("could not open tagged MP3: %v", tagErr)
	}
	defer tag.Close()

	t.Logf("ID3 tags: title=%q artist=%q album=%q", tag.Title(), tag.Artist(), tag.Album())
	if tag.Artist() == "" {
		t.Error("artist tag is empty")
	}
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	t.Logf("embedded pictures: %d", len(pics))
	if len(pics) == 0 {
		t.Error("no cover art embedded in MP3")
	}
}

func joinLines(lines []string) string {
	s := ""
	for _, l := range lines {
		s += "  " + l + "\n"
	}
	return s
}

func TestTagMP3_EmojiInMetadata(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "episode.mp3")
	os.WriteFile(mp3Path, minimalMP3(), 0644)

	coverPath := filepath.Join(dir, "cover.jpg")
	os.WriteFile(coverPath, []byte("fake-jpeg-data"), 0644)

	err := TagEpisode(mp3Path, "🎙️ My Podcast", "Episode 🔥 #42: The 🌍 Tour", coverPath)
	if err != nil {
		t.Fatalf("TagEpisode with emoji failed: %v", err)
	}

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("could not re-open tagged MP3: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "Episode 🔥 #42: The 🌍 Tour" {
		t.Errorf("title = %q, want emoji title", tag.Title())
	}
	if tag.Artist() != "🎙️ My Podcast" {
		t.Errorf("artist = %q, want emoji artist", tag.Artist())
	}
	if tag.Album() != "🎙️ My Podcast" {
		t.Errorf("album = %q, want emoji album", tag.Album())
	}

	// Verify cover art still embedded correctly with emoji metadata
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pics) == 0 {
		t.Error("no picture frames found — emoji metadata shouldn't break cover embedding")
	}
}

func TestTagMP3_UnicodeSpecialChars(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "episode.mp3")
	os.WriteFile(mp3Path, minimalMP3(), 0644)

	err := TagEpisode(mp3Path, "日本語ポッドキャスト", "第42話 — Björk's Jóga (Live)", "")
	if err != nil {
		t.Fatalf("TagEpisode with unicode failed: %v", err)
	}

	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("could not re-open: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "第42話 — Björk's Jóga (Live)" {
		t.Errorf("title = %q", tag.Title())
	}
	if tag.Artist() != "日本語ポッドキャスト" {
		t.Errorf("artist = %q", tag.Artist())
	}
}

func TestFetchCoverArt_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "cover.jpg")

	err := FetchCoverArt(srv.URL+"/art.jpg", dest)
	if err == nil {
		t.Error("expected error for 404 response")
	}
}
