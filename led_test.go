package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLEDControls(t *testing.T) {
	root := t.TempDir()
	ledDir := filepath.Join(root, stateLEDName)
	if err := os.MkdirAll(ledDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trigger", "brightness", "delay_on", "delay_off"} {
		if err := os.WriteFile(filepath.Join(ledDir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	prev := ledBasePath
	ledBasePath = root
	defer func() { ledBasePath = prev }()

	if err := SetLEDOn(); err != nil {
		t.Fatal(err)
	}
	assertFileEquals(t, filepath.Join(ledDir, "trigger"), "default-on")

	if err := SetLEDBlink(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	assertFileEquals(t, filepath.Join(ledDir, "trigger"), "timer")
	assertFileEquals(t, filepath.Join(ledDir, "delay_on"), "300")
	assertFileEquals(t, filepath.Join(ledDir, "delay_off"), "300")

	if err := SetLEDOff(); err != nil {
		t.Fatal(err)
	}
	assertFileEquals(t, filepath.Join(ledDir, "trigger"), "none")
	assertFileEquals(t, filepath.Join(ledDir, "brightness"), "0")

	if err := SetLEDHeartbeat(); err != nil {
		t.Fatal(err)
	}
	assertFileEquals(t, filepath.Join(ledDir, "trigger"), "heartbeat")
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}
