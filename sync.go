package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// syncDoneMsg is sent when rsync completes.
type syncDoneMsg struct {
	err    error
	output string // Full rsync output
}

// runSync runs rsync to copy new music from source to destination.
// Returns a tea.Cmd that collects all output and sends a syncDoneMsg.
func runSync(source, dest string) tea.Cmd {
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

		cmd := exec.Command("rsync", "-av", "--progress", source, dest)

		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return syncDoneMsg{err: fmt.Errorf("could not create pipe: %w", err)}
		}
		cmd.Stderr = cmd.Stdout // merge stderr into stdout

		if err := cmd.Start(); err != nil {
			return syncDoneMsg{err: fmt.Errorf("could not start rsync: %w", err)}
		}

		// Collect all output
		var lines []string
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		output := strings.Join(lines, "\n")
		err = cmd.Wait()
		if err != nil {
			return syncDoneMsg{err: fmt.Errorf("rsync failed: %w", err), output: output}
		}

		return syncDoneMsg{err: nil, output: output}
	}
}
