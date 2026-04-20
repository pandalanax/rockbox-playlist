package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runtimeStateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "rockbox-playlist")
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".local", "state", "rockbox-playlist")
	}
	return filepath.Join(os.TempDir(), "rockbox-playlist")
}

func runtimeMarkerPath(name string) string {
	return filepath.Join(runtimeStateDir(), name)
}

func activeSessionMarkerPath() string {
	return runtimeMarkerPath("session-active")
}

func autosyncSkipMarkerPath() string {
	return runtimeMarkerPath("skip-autosync")
}

func writeRuntimeMarker(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("1\n"), 0644)
}

func clearRuntimeMarker(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func hasRuntimeMarker(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func markSessionActive() error {
	return writeRuntimeMarker(activeSessionMarkerPath())
}

func clearSessionActive() error {
	return clearRuntimeMarker(activeSessionMarkerPath())
}

func sessionActive() bool {
	return hasRuntimeMarker(activeSessionMarkerPath())
}

func enableAutosyncSkip() error {
	return writeRuntimeMarker(autosyncSkipMarkerPath())
}

func disableAutosyncSkip() error {
	return clearRuntimeMarker(autosyncSkipMarkerPath())
}

func autosyncSkipEnabled() bool {
	return hasRuntimeMarker(autosyncSkipMarkerPath())
}

func autosyncSkipStatus() string {
	if autosyncSkipEnabled() {
		return fmt.Sprintf("enabled (%s)", autosyncSkipMarkerPath())
	}
	return fmt.Sprintf("disabled (%s)", autosyncSkipMarkerPath())
}
