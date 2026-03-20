package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// runDoctor scans the device for encoding issues in playlists and FLAC tags.
func runDoctor(playlistDir, musicDir string) {
	fmt.Println("Scanning for encoding issues...")
	fmt.Println()

	nfdFiles, nfdLines := checkNFDPlaylists(playlistDir)
	fmt.Println()
	tagFiles := checkFLACTags(musicDir)
	fmt.Println()

	// Summary
	if nfdLines == 0 && len(tagFiles) == 0 {
		fmt.Println("No issues found.")
		return
	}

	fmt.Println("--- Fix commands ---")
	fmt.Println()

	if nfdLines > 0 {
		fmt.Printf("Playlist fix (%d lines in %d files):\n", nfdLines, nfdFiles)
		fmt.Printf("  Install: sudo apt install icu-devtools\n")
		fmt.Printf("  Run:\n")
		fmt.Printf("    find %s -name '*.m3u8' -o -name '*.m3u' | while read f; do\n", playlistDir)
		fmt.Printf("      uconv -x nfc \"$f\" > \"$f.tmp\" && mv \"$f.tmp\" \"$f\"\n")
		fmt.Printf("    done\n")
		fmt.Println()
	}

	if len(tagFiles) > 0 {
		fmt.Printf("FLAC tag fix (%d files):\n", len(tagFiles))
		fmt.Printf("  Run:\n")
		fmt.Printf("    for f in")
		for _, f := range tagFiles {
			fmt.Printf(" \\\n      %q", f)
		}
		fmt.Printf("; do\n")
		fmt.Printf("      metaflac --export-tags-to=- \"$f\" | iconv -f latin1 -t utf-8 | metaflac --remove-all-tags --import-tags-from=- \"$f\"\n")
		fmt.Printf("    done\n")
		fmt.Println()
	}

	fmt.Println("After fixing, reinitialize the Rockbox database: Settings > General > Database > Initialize Now")
}

// checkNFDPlaylists scans playlist files for NFD-decomposed paths.
func checkNFDPlaylists(playlistDir string) (affectedFiles, affectedLines int) {
	fmt.Printf("Checking playlists in %s ...\n", playlistDir)

	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not read playlist directory: %v\n", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".m3u8" && ext != ".m3u" {
			continue
		}

		path := filepath.Join(playlistDir, entry.Name())
		lines := scanFileForNFD(path)
		if len(lines) > 0 {
			affectedFiles++
			affectedLines += len(lines)
			fmt.Printf("  %s (%d lines)\n", entry.Name(), len(lines))
			for _, l := range lines {
				fmt.Printf("    %d: %s\n", l.num, l.text)
			}
		}
	}

	if affectedFiles == 0 {
		fmt.Println("  No NFD issues found.")
	} else {
		fmt.Printf("  Found %d NFD lines in %d files.\n", affectedLines, affectedFiles)
	}
	return
}

type nfdLine struct {
	num  int
	text string
}

func scanFileForNFD(path string) []nfdLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []nfdLine
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if norm.NFC.String(line) != line {
			lines = append(lines, nfdLine{num: n, text: line})
		}
	}
	return lines
}

// checkFLACTags scans FLAC files for double-encoded UTF-8 tags.
func checkFLACTags(musicDir string) []string {
	fmt.Printf("Checking FLAC tags in %s ...\n", musicDir)

	metaflacPath, err := exec.LookPath("metaflac")
	if err != nil {
		fmt.Println("  metaflac not found. Install it to enable FLAC tag checking:")
		fmt.Println("    sudo apt install flac")
		return nil
	}
	_ = metaflacPath

	var affected []string

	filepath.Walk(musicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || strings.ToLower(filepath.Ext(path)) != ".flac" {
			return nil
		}

		cmd := exec.Command("metaflac", "--export-tags-to=-", path)
		out, err := cmd.Output()
		if err != nil {
			return nil
		}

		if hasDoubleEncodedUTF8(out) {
			rel, _ := filepath.Rel(musicDir, path)
			if rel == "" {
				rel = path
			}
			affected = append(affected, path)

			// Show the bad tags
			lines := bytes.Split(out, []byte("\n"))
			for _, line := range lines {
				if hasDoubleEncodedUTF8(line) {
					fmt.Printf("  %s: %s\n", rel, string(line))
				}
			}
		}
		return nil
	})

	if len(affected) == 0 {
		fmt.Println("  No double-encoded tags found.")
	} else {
		fmt.Printf("  Found %d files with double-encoded tags.\n", len(affected))
	}

	return affected
}

// hasDoubleEncodedUTF8 detects UTF-8 text that was treated as Latin-1 and
// re-encoded. The signature is a two-byte UTF-8 sequence (C3 xx) where each
// original byte was individually re-encoded, producing patterns like:
//   C3 83 C2 xx  (originally C3 xx, i.e. characters U+00C0..U+00FF)
// For example, "ú" (C3 BA) becomes "Ãº" (C3 83 C2 BA).
func hasDoubleEncodedUTF8(data []byte) bool {
	for i := 0; i < len(data)-3; i++ {
		if data[i] == 0xC3 && data[i+1] >= 0x80 && data[i+1] <= 0xBF &&
			data[i+2] == 0xC2 && data[i+3] >= 0x80 && data[i+3] <= 0xBF {
			return true
		}
	}
	return false
}
