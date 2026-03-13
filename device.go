package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// deviceStatusMsg is sent when device polling detects a status change.
type deviceStatusMsg struct {
	connected bool
	path      string // The device path (only set when connected)
}

// deviceEjectMsg is sent after an eject attempt completes.
type deviceEjectMsg struct {
	err error
}

// Common mount point patterns to scan when no explicit path is given.
var defaultDevicePaths = []string{
	"/Volumes/NO NAME",
	"/mnt/rockbox",
	"/media/*/NO NAME",
}

// CheckDevice checks whether a Rockbox player is mounted at the given path.
// It verifies the path exists and contains expected subdirectories.
func CheckDevice(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for at least one of the expected dirs
	for _, sub := range []string{"Music", "Playlists"} {
		subPath := filepath.Join(path, sub)
		if info, err := os.Stat(subPath); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

// FindDevicePath scans common mount locations for a Rockbox player.
// Returns the first matching path, or empty string if none found.
func FindDevicePath() string {
	for _, pattern := range defaultDevicePaths {
		if strings.Contains(pattern, "*") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, match := range matches {
				if CheckDevice(match) {
					return match
				}
			}
		} else {
			if CheckDevice(pattern) {
				return pattern
			}
		}
	}
	return ""
}

// WatchForDevice returns a tea.Cmd that polls for device presence.
// It checks every interval and sends a deviceStatusMsg.
func WatchForDevice(devicePath string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)

		if devicePath != "" {
			return deviceStatusMsg{
				connected: CheckDevice(devicePath),
				path:      devicePath,
			}
		}

		// Auto-detect
		found := FindDevicePath()
		return deviceStatusMsg{
			connected: found != "",
			path:      found,
		}
	}
}

// EjectDevice unmounts/ejects the device at the given path.
// Platform-aware: uses diskutil on macOS, umount on Linux.
func EjectDevice(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd

		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("diskutil", "eject", path)
		case "linux":
			cmd = exec.Command("umount", path)
		default:
			return deviceEjectMsg{
				err: fmt.Errorf("eject not supported on %s", runtime.GOOS),
			}
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			outStr := strings.TrimSpace(string(output))
			if outStr != "" {
				return deviceEjectMsg{
					err: fmt.Errorf("could not eject player: %s", outStr),
				}
			}
			return deviceEjectMsg{
				err: fmt.Errorf("could not eject player: %w", err),
			}
		}

		return deviceEjectMsg{err: nil}
	}
}
