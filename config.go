package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// AppConfig is the top-level configuration loaded from config.toml.
type AppConfig struct {
	Device  DeviceSettings  `toml:"device"`
	Sync    SyncSettings    `toml:"sync"`
	Music   MusicSettings   `toml:"music"`
	Backup  BackupSettings  `toml:"backup"`
	Podcast PodcastSettings `toml:"podcast"`
	Server  ServerSettings  `toml:"server"`
}

// DeviceSettings configures device detection and directory layout.
type DeviceSettings struct {
	Path          string   `toml:"path"`
	SearchPaths   []string `toml:"search_paths"`
	PlaylistDir   string   `toml:"playlist_dir"`
	MusicDir      string   `toml:"music_dir"`
	AudiobooksDir string   `toml:"audiobooks_dir"`
}

// SyncSettings configures music sync.
type SyncSettings struct {
	Source string `toml:"source"`
}

// MusicSettings configures music scanning.
type MusicSettings struct {
	RescanInterval string `toml:"rescan_interval"`
}

// BackupSettings configures playlist backups.
type BackupSettings struct {
	MaxBackups int `toml:"max_backups"`
}

// PodcastSettings configures podcast management.
type PodcastSettings struct {
	EpisodesToKeep int `toml:"episodes_to_keep"`
}

// ServerSettings configures the SSH server.
type ServerSettings struct {
	Host       string `toml:"host"`
	Port       string `toml:"port"`
	HostKeyDir string `toml:"host_key_dir"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() AppConfig {
	return AppConfig{
		Device: DeviceSettings{
			SearchPaths:   []string{"/Volumes/NO NAME", "/mnt/rockbox", "/media/*/NO NAME"},
			PlaylistDir:   "Playlists",
			MusicDir:      "Music",
			AudiobooksDir: "Audiobooks",
		},
		Music: MusicSettings{
			RescanInterval: "5s",
		},
		Backup: BackupSettings{
			MaxBackups: 10,
		},
		Podcast: PodcastSettings{
			EpisodesToKeep: 3,
		},
		Server: ServerSettings{
			Host:       "0.0.0.0",
			Port:       "2222",
			HostKeyDir: ".ssh",
		},
	}
}

// GetConfigPath returns the path to the config file, respecting XDG_CONFIG_HOME.
func GetConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rockbox-playlist", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rockbox-playlist", "config.toml")
}

// LoadConfig loads the configuration from the config file.
// Returns the default config if the file doesn't exist.
func LoadConfig() AppConfig {
	cfg := DefaultConfig()

	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg
	}

	// Decode TOML on top of defaults — only set fields override
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		// If the file exists but is malformed, use defaults
		return DefaultConfig()
	}

	return cfg
}

// RescanDuration parses the rescan interval string to a time.Duration.
// Falls back to 5 seconds if parsing fails.
func (c AppConfig) RescanDuration() time.Duration {
	d, err := time.ParseDuration(c.Music.RescanInterval)
	if err != nil || d <= 0 {
		return 5 * time.Second
	}
	return d
}
