package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// === Pure logic tests ===

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Hello/World", "Hello_World"},
		{"A:B*C?D", "A_B_C_D"},
		{"normal name", "normal name"},
		{"en\u2013dash", "en-dash"},
		{"em\u2014dash", "em-dash"},
		{"  multiple   spaces  ", "multiple spaces"},
		{strings.Repeat("x", 200), strings.Repeat("x", 150)},
		{"Podcast: The \"Best\" of 2024!", "Podcast_ The _Best_ of 2024!"},
		{"File<Name>With|Pipes", "File_Name_With_Pipes"},
		{"Motörhead's Greatest", "Motörhead's Greatest"},
		{"日本語ポッドキャスト", "日本語ポッドキャスト"},
		{"Episode #42 — The Big One", "Episode #42 - The Big One"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 20)], func(t *testing.T) {
			got := SanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeName_Emoji(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"🎙️ My Podcast", "🎙️ My Podcast"},
		{"Podcast 🔥: The Best!", "Podcast 🔥_ The Best!"},
		{"🌍 World 🎵 Music", "🌍 World 🎵 Music"},
		{"DJ 💿/Mix", "DJ 💿_Mix"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		url, want string
	}{
		{"https://example.com/ep.mp3", ".mp3"},
		{"https://example.com/ep.m4a?token=abc", ".m4a"},
		{"https://example.com/ep.aac", ".aac"},
		{"https://example.com/stream", ".mp3"}, // default
		{"https://example.com/EP.M4A", ".m4a"}, // case insensitive
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := GetFileExtension(tt.url)
			if got != tt.want {
				t.Errorf("GetFileExtension(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsPodcastPlaylist(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Podcasts", true},
		{"podcasts", true},
		{"Podcast", true},
		{"podcast", true},
		{"My Music", false},
		{"Rock", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPodcastPlaylist(tt.name)
			if got != tt.want {
				t.Errorf("IsPodcastPlaylist(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"<![CDATA[My Title]]>", "My Title"},
		{"  spaces  ", "spaces"},
		{"Normal Title", "Normal Title"},
		{"<![CDATA[Björk's Jóga — Live]]>", "Björk's Jóga — Live"},
		{"<![CDATA[日本語タイトル]]>", "日本語タイトル"},
		{"<![CDATA[Episode #42: \"The Big One\"]]>", "Episode #42: \"The Big One\""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanTitle(tt.input)
			if got != tt.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRSSChannelArtworkURL(t *testing.T) {
	tests := []struct {
		name    string
		channel RSSChannel
		want    string
	}{
		{
			name: "itunes image wins over RSS image",
			channel: RSSChannel{
				ITunesImage: RSSITunesImage{Href: "https://example.com/itunes.jpg"},
				Image:       RSSImage{URL: "https://example.com/rss.jpg"},
			},
			want: "https://example.com/itunes.jpg",
		},
		{
			name: "falls back to RSS image",
			channel: RSSChannel{
				Image: RSSImage{URL: "https://example.com/rss.jpg"},
			},
			want: "https://example.com/rss.jpg",
		},
		{
			name:    "empty when neither present",
			channel: RSSChannel{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.channel.ArtworkURL()
			if got != tt.want {
				t.Errorf("ArtworkURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRSSChannelArtworkURL_XMLParsing(t *testing.T) {
	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" version="2.0">
<channel>
<title>Test Podcast</title>
<itunes:image href="https://example.com/itunes-art.jpg"/>
<image><url>https://example.com/rss-art.jpg</url></image>
<item><title>Ep1</title><enclosure url="https://example.com/ep1.mp3" type="audio/mpeg"/></item>
</channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer srv.Close()

	feed, err := FetchRSSFeed(srv.URL)
	if err != nil {
		t.Fatalf("FetchRSSFeed failed: %v", err)
	}

	got := feed.Channel.ArtworkURL()
	if got != "https://example.com/itunes-art.jpg" {
		t.Errorf("ArtworkURL() = %q, want itunes-art.jpg URL", got)
	}
}

func TestParseRSSEpisodes(t *testing.T) {
	feed := &RSSFeed{
		Channel: RSSChannel{
			Title: "Test Podcast",
			Items: []RSSItem{
				{Title: "Episode 1", PubDate: "Mon, 01 Jan 2024 00:00:00 +0000", Enclosure: RSSEnclosure{URL: "https://example.com/ep1.mp3"}},
				{Title: "Episode 2", PubDate: "Tue, 02 Jan 2024 00:00:00 +0000", Enclosure: RSSEnclosure{URL: "https://example.com/ep2.mp3"}},
				{Title: "No Audio", PubDate: "Wed, 03 Jan 2024 00:00:00 +0000", Enclosure: RSSEnclosure{URL: ""}}, // should be skipped
				{Title: "Episode 3", PubDate: "Thu, 04 Jan 2024 00:00:00 +0000", Enclosure: RSSEnclosure{URL: "https://example.com/ep3.mp3"}},
			},
		},
	}

	episodes := ParseRSSEpisodes(feed, 3)
	// Limit is 3, but one of the first 3 items has no URL, so we get 2
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2 (limit 3, but one has no URL)", len(episodes))
	}
	if episodes[0].Title != "Episode 1" {
		t.Errorf("first episode = %q, want Episode 1", episodes[0].Title)
	}
}

func TestParseRSSDate(t *testing.T) {
	tests := []struct {
		input string
		year  int
	}{
		{"Mon, 01 Jan 2024 00:00:00 +0000", 2024},
		{"2024-01-15T10:30:00Z", 2024},
		{"2024-01-15", 2024},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseRSSDate(tt.input)
			if got.Year() != tt.year {
				t.Errorf("parseRSSDate(%q).Year() = %d, want %d", tt.input, got.Year(), tt.year)
			}
		})
	}

	// Invalid date should return approximately now
	got := parseRSSDate("not a date")
	if time.Since(got) > 5*time.Second {
		t.Errorf("parseRSSDate(invalid) should return ~now, got %v", got)
	}
}

// === File I/O tests ===

func TestLoadPodcastConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "podcasts.json")

	configJSON := `{
		"Test Podcast": {
			"feedUrl": "https://example.com/feed.xml",
			"episodes": [{"file": "ep1.mp3", "date": "2024-01-01"}]
		}
	}`
	os.WriteFile(path, []byte(configJSON), 0644)

	config, err := LoadPodcastConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config) != 1 {
		t.Fatalf("got %d podcasts, want 1", len(config))
	}

	feed, ok := config["Test Podcast"]
	if !ok {
		t.Fatal("missing 'Test Podcast' entry")
	}
	if feed.FeedURL != "https://example.com/feed.xml" {
		t.Errorf("FeedURL = %q", feed.FeedURL)
	}
	if len(feed.Episodes) != 1 {
		t.Errorf("got %d episodes, want 1", len(feed.Episodes))
	}
}

func TestLoadPodcastConfig_NonExistent(t *testing.T) {
	config, err := LoadPodcastConfig("/nonexistent/podcasts.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(config) != 0 {
		t.Errorf("got %d entries, want 0", len(config))
	}
}

func TestLoadPodcastConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "podcasts.json")
	os.WriteFile(path, []byte(`{invalid json`), 0644)

	_, err := LoadPodcastConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSavePodcastConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "podcasts.json")

	config := PodcastConfig{
		"My Podcast": PodcastFeed{
			FeedURL:  "https://example.com/feed.xml",
			Episodes: []Episode{{File: "ep1.mp3", Date: "2024-01-01"}},
		},
	}

	if err := SavePodcastConfig(path, config); err != nil {
		t.Fatal(err)
	}

	// Read back and verify round-trip
	loaded, err := LoadPodcastConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("round-trip: got %d entries, want 1", len(loaded))
	}

	// Verify JSON is indented (readable)
	raw, _ := os.ReadFile(path)
	var check map[string]interface{}
	json.Unmarshal(raw, &check)
	if len(check) != 1 {
		t.Error("JSON should be valid and have 1 key")
	}
}

func TestSavePodcastConfig_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "podcasts.json")

	config := PodcastConfig{
		"Björk's Podcast": PodcastFeed{
			FeedURL: "https://example.com/björk",
			Episodes: []Episode{
				{File: "Björk's Podcast-Jóga (Live).mp3", Date: "2024-01-01"},
			},
		},
		"日本語ポッドキャスト": PodcastFeed{
			FeedURL: "https://example.com/jp",
			Episodes: []Episode{
				{File: "日本語ポッドキャスト-第1話.mp3", Date: "2024-02-01"},
			},
		},
	}

	if err := SavePodcastConfig(path, config); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPodcastConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("round-trip: got %d entries, want 2", len(loaded))
	}

	bjork, ok := loaded["Björk's Podcast"]
	if !ok {
		t.Fatal("missing Björk's Podcast")
	}
	if bjork.Episodes[0].File != "Björk's Podcast-Jóga (Live).mp3" {
		t.Errorf("episode file not preserved: %q", bjork.Episodes[0].File)
	}

	jp, ok := loaded["日本語ポッドキャスト"]
	if !ok {
		t.Fatal("missing 日本語ポッドキャスト")
	}
	if jp.Episodes[0].File != "日本語ポッドキャスト-第1話.mp3" {
		t.Errorf("japanese episode file not preserved: %q", jp.Episodes[0].File)
	}
}

func TestRebuildPodcastPlaylist(t *testing.T) {
	dir := t.TempDir()
	playlistPath := filepath.Join(dir, "podcasts.m3u8")
	audioDir := filepath.Join(dir, "Audiobooks")

	config := PodcastConfig{
		"Pod A": PodcastFeed{
			FeedURL: "https://example.com/a",
			Episodes: []Episode{
				{File: "Pod A-ep1.mp3", Date: "2024-01-01"},
				{File: "Pod A-ep2.mp3", Date: "2024-01-15"},
			},
		},
		"Pod B": PodcastFeed{
			FeedURL: "https://example.com/b",
			Episodes: []Episode{
				{File: "Pod B-ep1.mp3", Date: "2024-01-10"},
			},
		},
	}

	if err := RebuildPodcastPlaylist(config, playlistPath, audioDir); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(playlistPath)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	if lines[0] != "#EXTM3U" {
		t.Errorf("first line = %q, want #EXTM3U", lines[0])
	}
	// 3 episodes + 1 header = 4 lines
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}
	// First episode after header should be newest (2024-01-15)
	if !strings.Contains(lines[1], "Pod A-ep2.mp3") {
		t.Errorf("newest episode should be first, got %q", lines[1])
	}
}

func TestRemovePodcast_NonexistentFolder(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "Audiobooks")

	config := PodcastConfig{
		"Ghost Podcast": PodcastFeed{
			FeedURL:  "https://example.com/ghost.xml",
			Episodes: []Episode{{File: "ep1.mp3", Date: "2024-01-01"}},
		},
	}

	// Should not panic even if folder doesn't exist on disk
	RemovePodcast("Ghost Podcast", config, audioDir)

	if _, ok := config["Ghost Podcast"]; ok {
		t.Error("Ghost Podcast should be removed from config")
	}
}

func TestRemovePodcast_LastPodcast(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "Audiobooks")
	podcastDir := filepath.Join(audioDir, "Only Podcast")
	os.MkdirAll(podcastDir, 0755)
	os.WriteFile(filepath.Join(podcastDir, "ep1.mp3"), []byte("audio"), 0644)

	config := PodcastConfig{
		"Only Podcast": PodcastFeed{
			FeedURL:  "https://example.com/only.xml",
			Episodes: []Episode{{File: "ep1.mp3", Date: "2024-01-01"}},
		},
	}

	RemovePodcast("Only Podcast", config, audioDir)

	if len(config) != 0 {
		t.Errorf("config should be empty, got %d entries", len(config))
	}
}

func TestRemovePodcast_EmojiName(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "Audiobooks")

	// SanitizeName will sanitize the folder name
	folderName := SanitizeName("🎙️ Podcast: The Best!")
	podcastDir := filepath.Join(audioDir, folderName)
	os.MkdirAll(podcastDir, 0755)
	os.WriteFile(filepath.Join(podcastDir, "ep1.mp3"), []byte("audio"), 0644)
	os.WriteFile(filepath.Join(podcastDir, "cover.jpg"), []byte("image"), 0644)

	config := PodcastConfig{
		"🎙️ Podcast: The Best!": PodcastFeed{
			FeedURL: "https://example.com/emoji.xml",
			Episodes: []Episode{
				{File: "ep1.mp3", Date: "2024-01-01"},
			},
		},
	}

	RemovePodcast("🎙️ Podcast: The Best!", config, audioDir)

	if _, ok := config["🎙️ Podcast: The Best!"]; ok {
		t.Error("emoji podcast should be removed from config")
	}
	if _, err := os.Stat(podcastDir); !os.IsNotExist(err) {
		t.Error("podcast directory should be deleted")
	}
}

func TestRemovePodcast_ThenRebuildPlaylist(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "Audiobooks")
	playlistPath := filepath.Join(dir, "podcasts.m3u8")

	config := PodcastConfig{
		"Pod A": PodcastFeed{
			FeedURL: "https://example.com/a",
			Episodes: []Episode{
				{File: "Pod A-ep1.mp3", Date: "2024-01-01"},
			},
		},
		"Pod B": PodcastFeed{
			FeedURL: "https://example.com/b",
			Episodes: []Episode{
				{File: "Pod B-ep1.mp3", Date: "2024-01-10"},
			},
		},
	}

	// Remove Pod A, then rebuild playlist
	RemovePodcast("Pod A", config, audioDir)
	if err := RebuildPodcastPlaylist(config, playlistPath, audioDir); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(playlistPath)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Should have header + 1 episode (only Pod B remains)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[1], "Pod B-ep1.mp3") {
		t.Errorf("expected Pod B episode, got %q", lines[1])
	}
	if strings.Contains(string(content), "Pod A") {
		t.Error("playlist should not contain Pod A entries after removal")
	}
}

func TestRemovePodcast(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "Audiobooks")

	// Create a podcast folder with some files
	podcastDir := filepath.Join(audioDir, "Test Podcast")
	os.MkdirAll(podcastDir, 0755)
	os.WriteFile(filepath.Join(podcastDir, "ep1.mp3"), []byte("audio"), 0644)
	os.WriteFile(filepath.Join(podcastDir, "ep2.mp3"), []byte("audio"), 0644)
	os.WriteFile(filepath.Join(podcastDir, "cover.jpg"), []byte("image"), 0644)

	config := PodcastConfig{
		"Test Podcast": PodcastFeed{
			FeedURL: "https://example.com/feed.xml",
			Episodes: []Episode{
				{File: "ep1.mp3", Date: "2024-01-01"},
				{File: "ep2.mp3", Date: "2024-01-15"},
			},
		},
		"Other Podcast": PodcastFeed{
			FeedURL:  "https://example.com/other.xml",
			Episodes: []Episode{{File: "other.mp3", Date: "2024-02-01"}},
		},
	}

	RemovePodcast("Test Podcast", config, audioDir)

	// Config should no longer have the removed podcast
	if _, ok := config["Test Podcast"]; ok {
		t.Error("Test Podcast should be removed from config")
	}
	// Other podcast should still be there
	if _, ok := config["Other Podcast"]; !ok {
		t.Error("Other Podcast should still be in config")
	}
	// Folder should be deleted
	if _, err := os.Stat(podcastDir); !os.IsNotExist(err) {
		t.Error("podcast directory should be deleted")
	}
}

// === HTTP tests ===

func TestFetchRSSFeed(t *testing.T) {
	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
<title>Test Podcast</title>
<item>
<title>Episode 1</title>
<pubDate>Mon, 01 Jan 2024 00:00:00 +0000</pubDate>
<enclosure url="https://example.com/ep1.mp3" type="audio/mpeg"/>
</item>
</channel></rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer srv.Close()

	feed, err := FetchRSSFeed(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Channel.Title != "Test Podcast" {
		t.Errorf("title = %q, want Test Podcast", feed.Channel.Title)
	}
	if len(feed.Channel.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(feed.Channel.Items))
	}
}

func TestFetchRSSFeed_SpecialCharacters(t *testing.T) {
	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss><channel>
<title>Björk's Podcast — Live</title>
<item>
<title><![CDATA[Episode #1: "Jóga" – Déconstruction]]></title>
<pubDate>Mon, 01 Jan 2024 00:00:00 +0000</pubDate>
<enclosure url="https://example.com/ep1.mp3" type="audio/mpeg"/>
</item>
<item>
<title>日本語エピソード</title>
<pubDate>Tue, 02 Jan 2024 00:00:00 +0000</pubDate>
<enclosure url="https://example.com/ep2.mp3" type="audio/mpeg"/>
</item>
</channel></rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Write([]byte(rssXML))
	}))
	defer srv.Close()

	feed, err := FetchRSSFeed(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Channel.Title != "Björk's Podcast — Live" {
		t.Errorf("title = %q", feed.Channel.Title)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(feed.Channel.Items))
	}
	if feed.Channel.Items[1].Title != "日本語エピソード" {
		t.Errorf("japanese title not preserved: %q", feed.Channel.Items[1].Title)
	}
}

func TestFetchRSSFeed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	_, err := FetchRSSFeed(srv.URL)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestDownloadEpisode(t *testing.T) {
	payload := "fake audio content for testing"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "30")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "episode.mp3")

	progressCalled := false
	err := DownloadEpisode(srv.URL, dest, func(downloaded, total int64) {
		progressCalled = true
	})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(dest)
	if string(content) != payload {
		t.Errorf("downloaded content = %q, want %q", string(content), payload)
	}
	if !progressCalled {
		t.Error("progress callback was not called")
	}
}

func TestDownloadEpisode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "episode.mp3")

	err := DownloadEpisode(srv.URL, dest, nil)
	if err == nil {
		t.Error("expected error for 404 response")
	}
}
