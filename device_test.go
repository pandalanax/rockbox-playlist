package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDevice_Valid(t *testing.T) {
	dir := createDeviceDir(t)
	if !CheckDevice(dir, "Music", "Playlists") {
		t.Error("CheckDevice should return true for valid device")
	}
}

func TestCheckDevice_DefaultSubDirs(t *testing.T) {
	dir := createDeviceDir(t)
	// No subDirs args — should use defaults (Music, Playlists)
	if !CheckDevice(dir) {
		t.Error("CheckDevice should return true with default subdirs")
	}
}

func TestCheckDevice_MissingSubDirs(t *testing.T) {
	dir := t.TempDir() // empty dir, no Music/ or Playlists/
	if CheckDevice(dir, "Music", "Playlists") {
		t.Error("CheckDevice should return false when subdirs are missing")
	}
}

func TestCheckDevice_NonExistentPath(t *testing.T) {
	if CheckDevice("/nonexistent/path/to/device") {
		t.Error("CheckDevice should return false for nonexistent path")
	}
}

func TestCheckDevice_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "afile")
	os.WriteFile(file, []byte("not a dir"), 0644)
	if CheckDevice(file) {
		t.Error("CheckDevice should return false when path is a file")
	}
}

func TestCheckDevice_CustomSubDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Audiobooks"), 0755)

	if !CheckDevice(dir, "Audiobooks") {
		t.Error("CheckDevice should return true when custom subdir exists")
	}
	if CheckDevice(dir, "NonExistent") {
		t.Error("CheckDevice should return false when custom subdir is missing")
	}
}

func TestFindDevicePath_Direct(t *testing.T) {
	device := createDeviceDir(t)
	noDevice := t.TempDir()

	found := FindDevicePath([]string{noDevice, device}, "Music", "Playlists")
	if found != device {
		t.Errorf("FindDevicePath = %q, want %q", found, device)
	}
}

func TestFindDevicePath_NoMatch(t *testing.T) {
	found := FindDevicePath([]string{"/nonexistent1", "/nonexistent2"}, "Music")
	if found != "" {
		t.Errorf("FindDevicePath should return empty string, got %q", found)
	}
}

func TestFindDevicePath_Glob(t *testing.T) {
	// Create a structure like /tmp/xxx/DEVICE/Music/
	parent := t.TempDir()
	deviceDir := filepath.Join(parent, "MYDEVICE")
	os.MkdirAll(filepath.Join(deviceDir, "Music"), 0755)

	pattern := filepath.Join(parent, "*")
	found := FindDevicePath([]string{pattern}, "Music")
	if found != deviceDir {
		t.Errorf("FindDevicePath glob = %q, want %q", found, deviceDir)
	}
}
