package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// httpGet performs an HTTP GET with a proper User-Agent header to avoid 403 errors
func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "rockbox-playlist/1.0")
	return http.DefaultClient.Do(req)
}

// PodcastConfig represents the podcasts.json file structure
type PodcastConfig map[string]PodcastFeed

// PodcastFeed represents a single podcast subscription
type PodcastFeed struct {
	FeedURL    string    `json:"feedUrl"`
	ArtworkURL string    `json:"artworkUrl,omitempty"`
	Episodes   []Episode `json:"episodes"`
}

// Episode represents a downloaded episode
type Episode struct {
	File string `json:"file"`
	Date string `json:"date"`
}

// iTunesSearchResult represents the iTunes API response
type iTunesSearchResult struct {
	ResultCount int             `json:"resultCount"`
	Results     []iTunesPodcast `json:"results"`
}

// iTunesPodcast represents a podcast from iTunes search
type iTunesPodcast struct {
	CollectionName string `json:"collectionName"`
	ArtistName     string `json:"artistName"`
	FeedURL        string `json:"feedUrl"`
	ArtworkURL     string `json:"artworkUrl100"`
}

// RSSFeed represents a parsed RSS feed
type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

// RSSChannel represents the channel element in RSS
type RSSChannel struct {
	Title       string         `xml:"title"`
	Items       []RSSItem      `xml:"item"`
	ITunesImage RSSITunesImage `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
	Image       RSSImage       `xml:"image"`
}

// RSSITunesImage represents the itunes:image element
type RSSITunesImage struct {
	Href string `xml:"href,attr"`
}

// RSSImage represents the standard RSS channel image
type RSSImage struct {
	URL string `xml:"url"`
}

// ArtworkURL returns the best artwork URL from the feed.
// Priority: itunes:image > standard RSS image.
func (ch *RSSChannel) ArtworkURL() string {
	if ch.ITunesImage.Href != "" {
		return ch.ITunesImage.Href
	}
	return ch.Image.URL
}

// RSSItem represents an item (episode) in RSS
type RSSItem struct {
	Title     string       `xml:"title"`
	PubDate   string       `xml:"pubDate"`
	Enclosure RSSEnclosure `xml:"enclosure"`
}

// RSSEnclosure represents the enclosure (audio file) in RSS
type RSSEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// RSSEpisode is a parsed episode ready for download
type RSSEpisode struct {
	Title   string
	URL     string
	PubDate time.Time
}

// LoadPodcastConfig loads the podcasts.json file
func LoadPodcastConfig(path string) (PodcastConfig, error) {
	config := make(PodcastConfig)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Empty config if file doesn't exist
		}
		return nil, fmt.Errorf("could not read podcast config: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("could not parse podcast config: %w", err)
	}

	return config, nil
}

// SavePodcastConfig saves the podcasts.json file
func SavePodcastConfig(path string, config PodcastConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode podcast config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("could not write podcast config: %w", err)
	}

	return nil
}

// SearchPodcasts searches iTunes for podcasts
func SearchPodcasts(query string) ([]iTunesPodcast, error) {
	escapedQuery := url.QueryEscape(query)
	apiURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&limit=5", escapedQuery)

	resp, err := httpGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("could not search podcasts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iTunes API returned status %d", resp.StatusCode)
	}

	var result iTunesSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not parse search results: %w", err)
	}

	return result.Results, nil
}

// FetchRSSFeed fetches and parses an RSS feed
func FetchRSSFeed(feedURL string) (*RSSFeed, error) {
	resp, err := httpGet(feedURL)
	if err != nil {
		return nil, fmt.Errorf("could not fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS feed returned status %d", resp.StatusCode)
	}

	var feed RSSFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("could not parse RSS feed: %w", err)
	}

	return &feed, nil
}

// ParseRSSEpisodes extracts the latest episodes from a feed
func ParseRSSEpisodes(feed *RSSFeed, limit int) []RSSEpisode {
	var episodes []RSSEpisode

	for i, item := range feed.Channel.Items {
		if i >= limit {
			break
		}

		if item.Enclosure.URL == "" {
			continue
		}

		pubDate := parseRSSDate(item.PubDate)
		episodes = append(episodes, RSSEpisode{
			Title:   cleanTitle(item.Title),
			URL:     item.Enclosure.URL,
			PubDate: pubDate,
		})
	}

	return episodes
}

// parseRSSDate parses various RSS date formats
func parseRSSDate(dateStr string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	return time.Now()
}

// cleanTitle removes CDATA and trims whitespace
func cleanTitle(title string) string {
	// Remove CDATA wrapper if present
	title = strings.TrimPrefix(title, "<![CDATA[")
	title = strings.TrimSuffix(title, "]]>")
	return strings.TrimSpace(title)
}

// SanitizeName cleans a string for use as a filename
func SanitizeName(s string) string {
	// Replace characters invalid in filenames
	re := regexp.MustCompile(`[\/\\:*?"<>|]`)
	s = re.ReplaceAllString(s, "_")

	// Replace en-dash and em-dash with regular dash
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")

	// Collapse multiple spaces
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Limit length
	if len(s) > 150 {
		s = s[:150]
	}

	return s
}

// GetFileExtension returns the audio file extension from a URL
func GetFileExtension(audioURL string) string {
	lower := strings.ToLower(audioURL)
	if strings.Contains(lower, ".m4a") {
		return ".m4a"
	}
	if strings.Contains(lower, ".aac") {
		return ".aac"
	}
	return ".mp3"
}

// DownloadEpisode downloads an episode to the specified path
func DownloadEpisode(audioURL, destPath string, onProgress func(downloaded, total int64)) error {
	resp, err := httpGet(audioURL)
	if err != nil {
		return fmt.Errorf("could not download episode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer out.Close()

	// Copy with progress
	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("could not write file: %w", writeErr)
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download error: %w", err)
		}
	}

	return nil
}

// UpdatePodcastWithLog updates a single podcast and returns detailed log messages
func UpdatePodcastWithLog(name string, feed *PodcastFeed, audioDir string, episodesToKeep int) (newCount int, deletedCount int, log []string, err error) {
	folderName := SanitizeName(name)
	podcastDir := filepath.Join(audioDir, folderName)

	// Create folder if needed
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		return 0, 0, log, fmt.Errorf("could not create podcast folder: %w", err)
	}

	// Fetch RSS
	rssFeed, err := FetchRSSFeed(feed.FeedURL)
	if err != nil {
		return 0, 0, log, err
	}

	// Backfill artwork URL from RSS if missing
	if feed.ArtworkURL == "" {
		feed.ArtworkURL = rssFeed.Channel.ArtworkURL()
	}

	// Get latest episodes from RSS
	rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

	// Build set of current files
	currentFiles := make(map[string]bool)
	for _, ep := range feed.Episodes {
		currentFiles[ep.File] = true
	}

	// Download new episodes
	var newEpisodes []Episode

	for _, ep := range rssEpisodes {
		ext := GetFileExtension(ep.URL)
		filename := SanitizeName(folderName+"-"+ep.Title) + ext
		epPath := filepath.Join(podcastDir, filename)
		dateStr := ep.PubDate.Format("2006-01-02")

		newEpisodes = append(newEpisodes, Episode{
			File: filename,
			Date: dateStr,
		})

		// Check if file already exists
		if _, statErr := os.Stat(epPath); statErr == nil {
			continue // Already have this episode
		}

		// Download
		log = append(log, fmt.Sprintf("  Downloading: %s", ep.Title))

		if dlErr := DownloadEpisode(ep.URL, epPath, nil); dlErr != nil {
			return newCount, deletedCount, log, dlErr
		}
		coverPath, _ := EnsureCoverArt(podcastDir, feed.ArtworkURL)
		_ = TagEpisode(epPath, name, ep.Title, coverPath)
		newCount++
	}

	// Sort by date descending
	sort.Slice(newEpisodes, func(i, j int) bool {
		return newEpisodes[i].Date > newEpisodes[j].Date
	})

	// Build set of files to keep
	filesToKeep := make(map[string]bool)
	for _, ep := range newEpisodes {
		filesToKeep[ep.File] = true
	}

	// Delete old episodes not in the new list
	for _, ep := range feed.Episodes {
		if !filesToKeep[ep.File] {
			oldPath := filepath.Join(podcastDir, ep.File)
			if _, statErr := os.Stat(oldPath); statErr == nil {
				log = append(log, fmt.Sprintf("  Deleting old: %s", ep.File))
				os.Remove(oldPath)
				deletedCount++
			}
		}
	}

	// Update episodes list
	feed.Episodes = newEpisodes

	return newCount, deletedCount, log, nil
}

// PendingEpisode represents an episode that would be downloaded during a podcast update
type PendingEpisode struct {
	PodcastName string
	Title       string
	URL         string
	DestPath    string
	Date        string
	ArtworkURL  string
}

// PendingDelete represents an episode that would be deleted during a podcast update
type PendingDelete struct {
	PodcastName string
	File        string
	Path        string
}

// PodcastCheckResult holds the dry-run results for all podcasts
type PodcastCheckResult struct {
	Downloads []PendingEpisode
	Deletes   []PendingDelete
	Errors    []string // per-podcast errors (non-fatal, we continue checking others)
}

// CheckPodcastUpdates performs a dry-run: fetches RSS feeds and determines what
// episodes would be downloaded and deleted, without actually doing anything.
func CheckPodcastUpdates(config PodcastConfig, audioDir string, episodesToKeep int) PodcastCheckResult {
	var result PodcastCheckResult

	for name := range config {
		feed := config[name]
		folderName := SanitizeName(name)
		podcastDir := filepath.Join(audioDir, folderName)

		// Fetch RSS
		rssFeed, err := FetchRSSFeed(feed.FeedURL)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		// Get latest episodes from RSS
		rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

		// Backfill artwork URL from RSS if missing
		artworkURL := feed.ArtworkURL
		if artworkURL == "" {
			artworkURL = rssFeed.Channel.ArtworkURL()
		}

		// Check which episodes are new (not on disk)
		for _, ep := range rssEpisodes {
			ext := GetFileExtension(ep.URL)
			filename := SanitizeName(folderName+"-"+ep.Title) + ext
			epPath := filepath.Join(podcastDir, filename)
			dateStr := ep.PubDate.Format("2006-01-02")

			if _, statErr := os.Stat(epPath); statErr != nil {
				// File doesn't exist — this is a new episode
				result.Downloads = append(result.Downloads, PendingEpisode{
					PodcastName: name,
					Title:       ep.Title,
					URL:         ep.URL,
					DestPath:    epPath,
					Date:        dateStr,
					ArtworkURL:  artworkURL,
				})
			}
		}

		// Check which episodes would be deleted
		filesToKeep := make(map[string]bool)
		for _, ep := range rssEpisodes {
			ext := GetFileExtension(ep.URL)
			filename := SanitizeName(folderName+"-"+ep.Title) + ext
			filesToKeep[filename] = true
		}

		for _, ep := range feed.Episodes {
			if !filesToKeep[ep.File] {
				oldPath := filepath.Join(podcastDir, ep.File)
				if _, statErr := os.Stat(oldPath); statErr == nil {
					result.Deletes = append(result.Deletes, PendingDelete{
						PodcastName: name,
						File:        ep.File,
						Path:        oldPath,
					})
				}
			}
		}
	}

	return result
}

// ExecutePodcastUpdates downloads new episodes and deletes old ones based on a check result.
// It also updates the config and rebuilds the playlist.
func ExecutePodcastUpdates(result PodcastCheckResult, config PodcastConfig, audioDir string, configPath string, playlistPath string, episodesToKeep int) (downloaded int, deleted int, err error) {
	// Download new episodes
	for _, ep := range result.Downloads {
		folderName := SanitizeName(ep.PodcastName)
		podcastDir := filepath.Join(audioDir, folderName)

		// Ensure folder exists
		if mkErr := os.MkdirAll(podcastDir, 0755); mkErr != nil {
			return downloaded, deleted, fmt.Errorf("could not create folder for %s: %w", ep.PodcastName, mkErr)
		}

		if dlErr := DownloadEpisode(ep.URL, ep.DestPath, nil); dlErr != nil {
			return downloaded, deleted, fmt.Errorf("downloading %s - %s: %w", ep.PodcastName, ep.Title, dlErr)
		}
		coverPath, _ := EnsureCoverArt(podcastDir, ep.ArtworkURL)
		_ = TagEpisode(ep.DestPath, ep.PodcastName, ep.Title, coverPath)
		downloaded++
	}

	// Delete old episodes
	for _, del := range result.Deletes {
		os.Remove(del.Path)
		deleted++
	}

	// Re-fetch and rebuild episode lists in config (since we downloaded new files)
	for name := range config {
		feed := config[name]
		folderName := SanitizeName(name)
		podcastDir := filepath.Join(audioDir, folderName)

		rssFeed, err := FetchRSSFeed(feed.FeedURL)
		if err != nil {
			continue // skip on error, config stays as-is
		}
		rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

		var newEpisodes []Episode
		for _, ep := range rssEpisodes {
			ext := GetFileExtension(ep.URL)
			filename := SanitizeName(folderName+"-"+ep.Title) + ext
			epPath := filepath.Join(podcastDir, filename)
			dateStr := ep.PubDate.Format("2006-01-02")

			// Only include if file actually exists on disk
			if _, statErr := os.Stat(epPath); statErr == nil {
				newEpisodes = append(newEpisodes, Episode{
					File: filename,
					Date: dateStr,
				})
			}
		}

		sort.Slice(newEpisodes, func(i, j int) bool {
			return newEpisodes[i].Date > newEpisodes[j].Date
		})

		feed.Episodes = newEpisodes
		config[name] = feed
	}

	// Save config
	if err := SavePodcastConfig(configPath, config); err != nil {
		return downloaded, deleted, err
	}

	// Rebuild playlist
	if err := RebuildPodcastPlaylist(config, playlistPath, audioDir); err != nil {
		return downloaded, deleted, err
	}

	return downloaded, deleted, nil
}

// UpdatePodcast updates a single podcast, downloading new episodes and deleting old ones
func UpdatePodcast(name string, feed *PodcastFeed, audioDir string, episodesToKeep int, onProgress func(string)) (int, error) {
	folderName := SanitizeName(name)
	podcastDir := filepath.Join(audioDir, folderName)

	// Create folder if needed
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		return 0, fmt.Errorf("could not create podcast folder: %w", err)
	}

	// Fetch RSS
	if onProgress != nil {
		onProgress(fmt.Sprintf("Fetching %s...", name))
	}

	rssFeed, err := FetchRSSFeed(feed.FeedURL)
	if err != nil {
		return 0, err
	}

	// Backfill artwork URL from RSS if missing
	if feed.ArtworkURL == "" {
		feed.ArtworkURL = rssFeed.Channel.ArtworkURL()
	}

	// Get latest episodes from RSS
	rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

	// Build set of current files
	currentFiles := make(map[string]bool)
	for _, ep := range feed.Episodes {
		currentFiles[ep.File] = true
	}

	// Download new episodes
	newCount := 0
	var newEpisodes []Episode

	for _, ep := range rssEpisodes {
		ext := GetFileExtension(ep.URL)
		filename := SanitizeName(folderName+"-"+ep.Title) + ext
		filepath := filepath.Join(podcastDir, filename)
		dateStr := ep.PubDate.Format("2006-01-02")

		newEpisodes = append(newEpisodes, Episode{
			File: filename,
			Date: dateStr,
		})

		// Check if file already exists
		if _, err := os.Stat(filepath); err == nil {
			continue // Already have this episode
		}

		// Download
		if onProgress != nil {
			onProgress(fmt.Sprintf("Downloading: %s", ep.Title))
		}

		if err := DownloadEpisode(ep.URL, filepath, nil); err != nil {
			return newCount, err
		}
		coverPath, _ := EnsureCoverArt(podcastDir, feed.ArtworkURL)
		_ = TagEpisode(filepath, name, ep.Title, coverPath)
		newCount++
	}

	// Sort by date descending
	sort.Slice(newEpisodes, func(i, j int) bool {
		return newEpisodes[i].Date > newEpisodes[j].Date
	})

	// Build set of files to keep
	filesToKeep := make(map[string]bool)
	for _, ep := range newEpisodes {
		filesToKeep[ep.File] = true
	}

	// Delete old episodes not in the new list
	for _, ep := range feed.Episodes {
		if !filesToKeep[ep.File] {
			oldPath := filepath.Join(podcastDir, ep.File)
			if _, err := os.Stat(oldPath); err == nil {
				if onProgress != nil {
					onProgress(fmt.Sprintf("Deleting old: %s", ep.File))
				}
				os.Remove(oldPath)
			}
		}
	}

	// Update episodes list
	feed.Episodes = newEpisodes

	return newCount, nil
}

// UpdateAllPodcasts updates all podcasts in the config
func UpdateAllPodcasts(config PodcastConfig, audioDir string, episodesToKeep int, onProgress func(string)) error {
	for name := range config {
		feed := config[name]
		_, err := UpdatePodcast(name, &feed, audioDir, episodesToKeep, onProgress)
		if err != nil {
			if onProgress != nil {
				onProgress(fmt.Sprintf("Error updating %s: %v", name, err))
			}
			// Continue with other podcasts
			continue
		}
		config[name] = feed
	}
	return nil
}

// AddPodcastWithLog adds a new podcast and returns detailed log messages
func AddPodcastWithLog(podcast iTunesPodcast, audioDir string, config PodcastConfig, episodesToKeep int) (log []string, err error) {
	name := podcast.CollectionName
	folderName := SanitizeName(name)
	podcastDir := filepath.Join(audioDir, folderName)

	// Create folder
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		return log, fmt.Errorf("could not create podcast folder: %w", err)
	}

	log = append(log, fmt.Sprintf("Adding: %s", name))
	log = append(log, fmt.Sprintf("Saving to: %s/", folderName))
	log = append(log, "")
	log = append(log, "Fetching episodes...")

	// Fetch RSS
	rssFeed, err := FetchRSSFeed(podcast.FeedURL)
	if err != nil {
		return log, err
	}

	// Determine artwork URL: iTunes > RSS feed
	artworkURL := podcast.ArtworkURL
	if artworkURL == "" {
		artworkURL = rssFeed.Channel.ArtworkURL()
	}

	// Get latest episodes
	rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

	var episodes []Episode
	downloadCount := 0

	for _, ep := range rssEpisodes {
		ext := GetFileExtension(ep.URL)
		filename := SanitizeName(folderName+"-"+ep.Title) + ext
		epPath := filepath.Join(podcastDir, filename)
		dateStr := ep.PubDate.Format("2006-01-02")

		episodes = append(episodes, Episode{
			File: filename,
			Date: dateStr,
		})

		// Check if already exists
		if _, statErr := os.Stat(epPath); statErr == nil {
			log = append(log, fmt.Sprintf("Skipping: %s (exists)", ep.Title))
			continue
		}

		// Download
		log = append(log, fmt.Sprintf("Downloading: %s", ep.Title))

		if dlErr := DownloadEpisode(ep.URL, epPath, nil); dlErr != nil {
			return log, dlErr
		}
		coverPath, coverErr := EnsureCoverArt(podcastDir, artworkURL)
		if coverErr != nil {
			log = append(log, fmt.Sprintf("  Cover art: %v", coverErr))
		}
		if tagErr := TagEpisode(epPath, name, ep.Title, coverPath); tagErr != nil {
			log = append(log, fmt.Sprintf("  Tagging: %v", tagErr))
		}
		log = append(log, fmt.Sprintf("Saved: %s/%s", folderName, filename))
		downloadCount++
	}

	// Sort by date descending
	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].Date > episodes[j].Date
	})

	// Add to config
	config[name] = PodcastFeed{
		FeedURL:    podcast.FeedURL,
		ArtworkURL: artworkURL,
		Episodes:   episodes,
	}

	log = append(log, "")
	log = append(log, fmt.Sprintf("Downloaded %d episodes", downloadCount))
	log = append(log, "Done!")

	return log, nil
}

// AddPodcast adds a new podcast from an iTunes result
func AddPodcast(podcast iTunesPodcast, audioDir string, config PodcastConfig, episodesToKeep int, onProgress func(string)) error {
	name := podcast.CollectionName
	folderName := SanitizeName(name)
	podcastDir := filepath.Join(audioDir, folderName)

	// Create folder
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		return fmt.Errorf("could not create podcast folder: %w", err)
	}

	if onProgress != nil {
		onProgress(fmt.Sprintf("Fetching episodes for %s...", name))
	}

	// Fetch RSS
	rssFeed, err := FetchRSSFeed(podcast.FeedURL)
	if err != nil {
		return err
	}

	// Determine artwork URL: iTunes > RSS feed
	artworkURL := podcast.ArtworkURL
	if artworkURL == "" {
		artworkURL = rssFeed.Channel.ArtworkURL()
	}

	// Get latest episodes
	rssEpisodes := ParseRSSEpisodes(rssFeed, episodesToKeep)

	var episodes []Episode
	for _, ep := range rssEpisodes {
		ext := GetFileExtension(ep.URL)
		filename := SanitizeName(folderName+"-"+ep.Title) + ext
		filepath := filepath.Join(podcastDir, filename)
		dateStr := ep.PubDate.Format("2006-01-02")

		episodes = append(episodes, Episode{
			File: filename,
			Date: dateStr,
		})

		// Check if already exists
		if _, err := os.Stat(filepath); err == nil {
			if onProgress != nil {
				onProgress(fmt.Sprintf("Skipping: %s (exists)", ep.Title))
			}
			continue
		}

		// Download
		if onProgress != nil {
			onProgress(fmt.Sprintf("Downloading: %s", ep.Title))
		}

		if err := DownloadEpisode(ep.URL, filepath, nil); err != nil {
			return err
		}
		coverPath, _ := EnsureCoverArt(podcastDir, artworkURL)
		_ = TagEpisode(filepath, name, ep.Title, coverPath)
	}

	// Sort by date descending
	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].Date > episodes[j].Date
	})

	// Add to config
	config[name] = PodcastFeed{
		FeedURL:    podcast.FeedURL,
		ArtworkURL: artworkURL,
		Episodes:   episodes,
	}

	return nil
}

// RebuildPodcastPlaylist regenerates the podcast playlist file
func RebuildPodcastPlaylist(config PodcastConfig, playlistPath, audioDir string) error {
	// Collect all episodes with dates
	type episodeEntry struct {
		date string
		path string
	}
	var allEpisodes []episodeEntry

	for name, feed := range config {
		folderName := SanitizeName(name)
		for _, ep := range feed.Episodes {
			// Path relative to Playlists folder
			relPath := fmt.Sprintf("../Audiobooks/%s/%s", folderName, ep.File)
			allEpisodes = append(allEpisodes, episodeEntry{
				date: ep.Date,
				path: relPath,
			})
		}
	}

	// Sort by date descending (newest first)
	sort.Slice(allEpisodes, func(i, j int) bool {
		return allEpisodes[i].date > allEpisodes[j].date
	})

	// Write playlist
	var lines []string
	lines = append(lines, "#EXTM3U")
	for _, ep := range allEpisodes {
		lines = append(lines, ep.path)
	}

	content := norm.NFC.String(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(playlistPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("could not write playlist: %w", err)
	}

	return nil
}

// RemovePodcast removes a podcast: deletes its folder from disk and removes it from the config map.
// The caller is responsible for saving the config and rebuilding the playlist.
func RemovePodcast(name string, config PodcastConfig, audioDir string) {
	folderName := SanitizeName(name)
	podcastDir := filepath.Join(audioDir, folderName)
	os.RemoveAll(podcastDir)
	delete(config, name)
}

// IsPodcastPlaylist checks if a playlist name indicates it's a podcast playlist
func IsPodcastPlaylist(name string) bool {
	lower := strings.ToLower(name)
	return lower == "podcasts" || lower == "podcast"
}
