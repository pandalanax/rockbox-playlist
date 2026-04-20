package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// syncDryRunMsg is sent when rsync dry-run completes.
type syncDryRunMsg struct {
	err   error
	files []string // Files that would be transferred
}

// syncDoneMsg is sent when rsync completes.
type syncDoneMsg struct {
	err   error
	count int // Number of files synced
}

// SyncPreview returns the relative file paths that rsync would transfer.
// Uses the same flags as the TUI dry-run flow.
func SyncPreview(source, dest string) ([]string, error) {
	// Ensure trailing slashes so rsync copies contents, not the directory
	if !strings.HasSuffix(source, "/") {
		source += "/"
	}
	if !strings.HasSuffix(dest, "/") {
		dest += "/"
	}

	// Check that rsync is available
	if _, err := exec.LookPath("rsync"); err != nil {
		return nil, fmt.Errorf("rsync not found. Please install rsync")
	}

	cmd := exec.Command("rsync", "-r", "--size-only", "--dry-run", "--out-format=%n", source, dest)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("could not create pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start rsync: %w", err)
	}

	var files []string
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasSuffix(line, "/") {
			files = append(files, line)
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("rsync dry-run failed: %w", err)
	}

	return files, nil
}

// SyncFiles copies files using the same rsync behavior as the TUI flow.
func SyncFiles(source, dest string) error {
	// Ensure trailing slashes so rsync copies contents, not the directory
	if !strings.HasSuffix(source, "/") {
		source += "/"
	}
	if !strings.HasSuffix(dest, "/") {
		dest += "/"
	}

	// Check that rsync is available
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found. Please install rsync")
	}

	args := []string{"-r", "--size-only"}

	// --info=progress2 is not supported by macOS openrsync, detect and skip
	check := exec.Command("rsync", "--info=progress2", "--version")
	if check.Run() == nil {
		args = append(args, "--info=progress2", "--no-inc-recursive")
	}

	args = append(args, source, dest)
	cmd := exec.Command("rsync", args...)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not create pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start rsync: %w", err)
	}

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	return nil
}

// runSyncDryRun runs rsync in dry-run mode to preview what would be synced.
// Returns files that would be transferred (only new files by size).
func runSyncDryRun(source, dest string) tea.Cmd {
	return func() tea.Msg {
		files, err := SyncPreview(source, dest)
		return syncDryRunMsg{err: err, files: files}
	}
}

// runSync runs rsync to copy new music from source to destination.
// Uses --size-only to only copy files that differ in size (not timestamps/permissions).
func runSync(source, dest string, expectedCount int) tea.Cmd {
	return func() tea.Msg {
		err := SyncFiles(source, dest)
		return syncDoneMsg{err: err, count: expectedCount}
	}
}
