package main

import (
	"os"
	"testing"
)

func TestAutosyncSkipMarkerLifecycle(t *testing.T) {
	_ = disableAutosyncSkip()
	defer func() { _ = disableAutosyncSkip() }()

	if autosyncSkipEnabled() {
		t.Fatal("autosync skip should start disabled")
	}

	if err := enableAutosyncSkip(); err != nil {
		t.Fatal(err)
	}
	if !autosyncSkipEnabled() {
		t.Fatal("autosync skip should be enabled")
	}

	if err := disableAutosyncSkip(); err != nil {
		t.Fatal(err)
	}
	if autosyncSkipEnabled() {
		t.Fatal("autosync skip should be disabled")
	}
}

func TestSessionMarkerLifecycle(t *testing.T) {
	_ = clearSessionActive()
	defer func() { _ = clearSessionActive() }()

	if sessionActive() {
		t.Fatal("session should start inactive")
	}

	if err := markSessionActive(); err != nil {
		t.Fatal(err)
	}
	if !sessionActive() {
		t.Fatal("session should be active")
	}

	if err := clearSessionActive(); err != nil {
		t.Fatal(err)
	}
	if sessionActive() {
		t.Fatal("session should be inactive")
	}
}

func TestRuntimeMarkersAreCreatedInTempDir(t *testing.T) {
	if path := runtimeStateDir(); path == "" {
		t.Fatal("runtime state dir should not be empty")
	}
	if _, err := os.Stat(os.TempDir()); err != nil {
		t.Fatal(err)
	}
}
