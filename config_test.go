package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Device.SearchPaths) != 3 {
		t.Errorf("SearchPaths has %d entries, want 3", len(cfg.Device.SearchPaths))
	}
	if cfg.Device.PlaylistDir != "Playlists" {
		t.Errorf("PlaylistDir = %q, want Playlists", cfg.Device.PlaylistDir)
	}
	if cfg.Device.MusicDir != "Music" {
		t.Errorf("MusicDir = %q, want Music", cfg.Device.MusicDir)
	}
	if cfg.Backup.MaxBackups != 10 {
		t.Errorf("MaxBackups = %d, want 10", cfg.Backup.MaxBackups)
	}
	if cfg.Podcast.EpisodesToKeep != 3 {
		t.Errorf("EpisodesToKeep = %d, want 3", cfg.Podcast.EpisodesToKeep)
	}
	if cfg.Server.Port != "2222" {
		t.Errorf("Port = %q, want 2222", cfg.Server.Port)
	}
}

func TestRescanDuration(t *testing.T) {
	tests := []struct {
		interval string
		want     time.Duration
	}{
		{"5s", 5 * time.Second},
		{"1m", time.Minute},
		{"", 5 * time.Second},
		{"invalid", 5 * time.Second},
		{"-1s", 5 * time.Second},
		{"0s", 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			cfg := AppConfig{Music: MusicSettings{RescanInterval: tt.interval}}
			got := cfg.RescanDuration()
			if got != tt.want {
				t.Errorf("RescanDuration(%q) = %v, want %v", tt.interval, got, tt.want)
			}
		})
	}
}

func TestGetConfigPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got := GetConfigPath()
	want := "/custom/config/rockbox-playlist/config.toml"
	if got != want {
		t.Errorf("GetConfigPath() = %q, want %q", got, want)
	}
}

func TestGetConfigPath_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got := GetConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("GetConfigPath() should return absolute path, got %q", got)
	}
	if filepath.Base(got) != "config.toml" {
		t.Errorf("GetConfigPath() should end with config.toml, got %q", got)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := LoadConfig()
	def := DefaultConfig()
	if cfg.Backup.MaxBackups != def.Backup.MaxBackups {
		t.Errorf("MaxBackups = %d, want default %d", cfg.Backup.MaxBackups, def.Backup.MaxBackups)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	dir := filepath.Join(configDir, "rockbox-playlist")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
[backup]
max_backups = 5

[podcast]
episodes_to_keep = 7
`), 0644)

	cfg := LoadConfig()
	if cfg.Backup.MaxBackups != 5 {
		t.Errorf("MaxBackups = %d, want 5", cfg.Backup.MaxBackups)
	}
	if cfg.Podcast.EpisodesToKeep != 7 {
		t.Errorf("EpisodesToKeep = %d, want 7", cfg.Podcast.EpisodesToKeep)
	}
	// Unset fields should keep defaults
	if cfg.Device.MusicDir != "Music" {
		t.Errorf("MusicDir should be default, got %q", cfg.Device.MusicDir)
	}
}

func TestLoadConfig_MalformedFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	dir := filepath.Join(configDir, "rockbox-playlist")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`{not valid toml!!!`), 0644)

	cfg := LoadConfig()
	def := DefaultConfig()
	if cfg.Backup.MaxBackups != def.Backup.MaxBackups {
		t.Errorf("malformed config should return defaults, got MaxBackups=%d", cfg.Backup.MaxBackups)
	}
}
