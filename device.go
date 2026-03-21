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

// CheckDevice checks whether a Rockbox player is mounted at the given path.
// It verifies the path exists and contains expected subdirectories.
// Uses the provided directory names (e.g. "Music", "Playlists") for detection.
func CheckDevice(path string, subDirs ...string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	if len(subDirs) == 0 {
		subDirs = []string{"Music", "Playlists"}
	}

	// Check for at least one of the expected dirs
	for _, sub := range subDirs {
		subPath := filepath.Join(path, sub)
		if info, err := os.Stat(subPath); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

// FindDevicePath scans the given search paths for a Rockbox player.
// Returns the first matching path, or empty string if none found.
func FindDevicePath(searchPaths []string, subDirs ...string) string {
	for _, pattern := range searchPaths {
		if strings.Contains(pattern, "*") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, match := range matches {
				if CheckDevice(match, subDirs...) {
					return match
				}
			}
		} else {
			if CheckDevice(pattern, subDirs...) {
				return pattern
			}
		}
	}
	return ""
}

// WatchForDevice returns a tea.Cmd that polls for device presence.
// It checks every interval and sends a deviceStatusMsg.
func WatchForDevice(devicePath string, searchPaths []string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)

		if devicePath != "" {
			return deviceStatusMsg{
				connected: CheckDevice(devicePath),
				path:      devicePath,
			}
		}

		// Auto-detect
		found := FindDevicePath(searchPaths)
		return deviceStatusMsg{
			connected: found != "",
			path:      found,
		}
	}
}

// EjectDevice unmounts/ejects the device at the given path.
// Platform-aware: uses diskutil on macOS, udisksctl on Linux.
// On Linux, resolves the mount point to a block device via findmnt,
// then unmounts with udisksctl which works without sudo via polkit.
func EjectDevice(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd

		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("diskutil", "eject", path)
		case "linux":
			// Resolve mount point to block device for udisksctl.
			out, err := exec.Command("findmnt", "-n", "-o", "SOURCE", path).Output()
			if err != nil {
				return deviceEjectMsg{
					err: fmt.Errorf("could not find block device for %s: %w", path, err),
				}
			}
			blockDev := strings.TrimSpace(string(out))
			if blockDev == "" {
				return deviceEjectMsg{
					err: fmt.Errorf("no block device found for mount point %s", path),
				}
			}
			cmd = exec.Command("udisksctl", "unmount", "-b", blockDev)
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
