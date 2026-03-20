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

// runSyncDryRun runs rsync in dry-run mode to preview what would be synced.
// Returns files that would be transferred (only new files by size).
func runSyncDryRun(source, dest string) tea.Cmd {
	return func() tea.Msg {
		// Ensure trailing slashes so rsync copies contents, not the directory
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
		if !strings.HasSuffix(dest, "/") {
			dest += "/"
		}

		// Check that rsync is available
		if _, err := exec.LookPath("rsync"); err != nil {
			return syncDryRunMsg{err: fmt.Errorf("rsync not found. Please install rsync")}
		}

		// Dry-run to see what would be transferred
		// --size-only: skip files that match in size (ignore timestamps/permissions)
		// --no-inc-recursive: build file list before transfer (needed for accurate count)
		cmd := exec.Command("rsync", "-r", "--size-only", "--dry-run", "--out-format=%n", source, dest)

		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return syncDryRunMsg{err: fmt.Errorf("could not create pipe: %w", err)}
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			return syncDryRunMsg{err: fmt.Errorf("could not start rsync: %w", err)}
		}

		// Collect file paths that would be transferred
		var files []string
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Skip empty lines and directory entries (ending with /)
			if line != "" && !strings.HasSuffix(line, "/") {
				files = append(files, line)
			}
		}

		if err := cmd.Wait(); err != nil {
			return syncDryRunMsg{err: fmt.Errorf("rsync dry-run failed: %w", err)}
		}

		return syncDryRunMsg{err: nil, files: files}
	}
}

// runSync runs rsync to copy new music from source to destination.
// Uses --size-only to only copy files that differ in size (not timestamps/permissions).
func runSync(source, dest string, expectedCount int) tea.Cmd {
	return func() tea.Msg {
		// Ensure trailing slashes so rsync copies contents, not the directory
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
		if !strings.HasSuffix(dest, "/") {
			dest += "/"
		}

		// Check that rsync is available
		if _, err := exec.LookPath("rsync"); err != nil {
			return syncDoneMsg{err: fmt.Errorf("rsync not found. Please install rsync")}
		}

		// Actual sync
		// -r: recursive
		// --size-only: skip files that match in size
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
			return syncDoneMsg{err: fmt.Errorf("could not create pipe: %w", err)}
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			return syncDoneMsg{err: fmt.Errorf("could not start rsync: %w", err)}
		}

		// Read all output (we don't display it live, just wait for completion)
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			// Just consume output
		}

		if err := cmd.Wait(); err != nil {
			return syncDoneMsg{err: fmt.Errorf("rsync failed: %w", err)}
		}

		return syncDoneMsg{err: nil, count: expectedCount}
	}
}
