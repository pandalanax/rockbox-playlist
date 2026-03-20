package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	id3v2 "github.com/bogem/id3v2/v2"
)

// FetchCoverArt downloads cover art from the given URL and saves it to destPath.
func FetchCoverArt(artworkURL, destPath string) error {
	resp, err := httpGet(artworkURL)
	if err != nil {
		return fmt.Errorf("could not fetch cover art: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cover art returned status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create cover art file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("could not write cover art: %w", err)
	}
	return nil
}

// EnsureCoverArt ensures cover.jpg exists in podcastDir.
// Returns the path to the cover art file, or "" if no artwork is available.
func EnsureCoverArt(podcastDir, artworkURL string) (string, error) {
	if artworkURL == "" {
		return "", nil
	}

	coverPath := filepath.Join(podcastDir, "cover.jpg")
	if _, err := os.Stat(coverPath); err == nil {
		return coverPath, nil
	}

	if err := FetchCoverArt(artworkURL, coverPath); err != nil {
		return "", err
	}
	return coverPath, nil
}

// TagEpisode writes metadata tags to an audio file.
// Only MP3 files are tagged; other formats are silently skipped.
// coverArtPath may be empty if no cover art is available.
func TagEpisode(filePath, podcastName, episodeTitle, coverArtPath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return tagMP3(filePath, podcastName, episodeTitle, coverArtPath)
	default:
		return nil
	}
}

func tagMP3(filePath, podcastName, episodeTitle, coverArtPath string) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: false})
	if err != nil {
		return fmt.Errorf("could not open MP3 for tagging: %w", err)
	}
	defer tag.Close()

	tag.SetDefaultEncoding(id3v2.EncodingUTF16)
	tag.SetTitle(episodeTitle)
	tag.SetArtist(podcastName)
	tag.SetAlbum(podcastName)

	if coverArtPath != "" {
		artData, err := os.ReadFile(coverArtPath)
		if err == nil {
			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingUTF16,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Cover",
				Picture:     artData,
			}
			tag.AddAttachedPicture(pic)
		}
	}

	if err := tag.Save(); err != nil {
		return fmt.Errorf("could not save MP3 tags: %w", err)
	}
	return nil
}
