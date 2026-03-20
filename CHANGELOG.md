# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] - 2026-03-20

### Added
- Remove podcast: delete a subscribed podcast from the podcast menu
  - Removes episode folder from disk, updates config, rebuilds playlist
  - Danger confirmation prompt before deletion
- Episode tagging: MP3 episodes now get ID3v2 metadata (title, artist, album) and embedded cover art
- Podcast artwork: cover art fetched from iTunes or RSS feed, cached as cover.jpg per podcast
- Recently Added playlist: press `S` (shift+s) on playlist picker to sync music and auto-add to Recently Added
  - Keeps last 100 entries, trims oldest when exceeded
- Comprehensive test suite covering all modules with edge cases (emoji, unicode, overflow)

### Fixed
- rsync compatibility with macOS openrsync (skip unsupported `--info=progress2` flag)

## [0.6.4] - 2026-03-17

### Added
- `config` subcommand: run `rockbox-playlist config` to print the default config.toml to stdout
- Embedded default config.toml in the binary

## [0.6.3] - 2026-03-16

### Changed
- Podcast update now uses dry-run preview with confirmation before downloading
  - Shows pending downloads and deletions before committing
  - Press `y` to proceed or `n`/`esc` to cancel

## [0.6.2] - 2026-03-14

### Added
- Playlist creation: press `c` on playlist picker to create a new playlist
- Playlist deletion: press `d` on playlist picker to delete selected playlist (with y/n confirmation)
- Sync dry-run preview: sync now shows files to be copied and asks for confirmation before proceeding
- Sync uses `--size-only` for faster rsync comparisons

### Changed
- Filter macOS `._` resource fork files from playlist listings
- Clean up dead code and inconsistencies

## [0.6.0] - 2026-03-14

### Added
- Configuration file support: `~/.config/rockbox-playlist/config.toml`
  - All paths and settings now configurable via TOML config file
  - Precedence: CLI flags > environment variables > config file > built-in defaults
  - Respects `$XDG_CONFIG_HOME` if set
  - Uses built-in defaults silently if no config file exists
- Configurable settings:
  - `device.path`: explicit device mount path
  - `device.search_paths`: paths to scan during auto-detect
  - `device.playlist_dir`, `music_dir`, `audiobooks_dir`: directory names on device
  - `sync.source`: source directory for music sync
  - `music.rescan_interval`: how often to rescan music library (e.g. "5s", "10s", "1m")
  - `backup.max_backups`: number of backups to keep
  - `podcast.episodes_to_keep`: episodes per podcast
  - `server.host`, `port`, `host_key_dir`: SSH server settings

### Changed
- Previously hardcoded values (rescan interval, max backups, episodes to keep, search paths, directory names) are now read from config
- CLI flag defaults are populated from config file values

## [0.5.0] - 2026-03-14

### Added
- Music sync feature: press `s` on playlist picker to sync music from a source directory
  - Runs rsync to copy new files from source to Rockbox Music folder
  - One-way sync, no deletes (only adds new files)
  - Detailed rsync output shown in TUI after completion
  - Configure via `--sync-source` flag or `ROCKBOX_SYNC_SOURCE` env var
  - Handles missing source with retry prompt (y/n)
  - `s: sync` shortcut shown in help keys only when source is configured

## [0.4.0] - 2026-03-13

### Added
- SSH server mode: run with `--serve` to expose the TUI over SSH (Wish)
  - Single-session access: only one SSH connection at a time
  - Configurable via flags: `--host`, `--port`, `--host-key-dir`, `--device-path`
  - Environment variable fallbacks: `ROCKBOX_SSH_PORT`, `ROCKBOX_DEVICE_PATH`
  - Auto-generates SSH host keys on first run
- Live music scanning: the music library is re-scanned every 5 seconds in the background
  - New songs added to the device appear automatically without restarting
  - Song browser refreshes in-place when new music is detected

### Changed
- CLI now uses `flag` package: positional args after flags (`rockbox-playlist [flags] [playlist-dir] [music-dir]`)

## [0.3.0] - 2026-03-06

### Added
- Device management: app now starts even when no Rockbox player is connected
  - Shows "No Rockbox player detected" screen with automatic polling
  - Auto-detects device when plugged in (checks /Volumes/NO NAME and other common paths)
  - Press `u` on playlist picker to eject the player (with toast-style y/n confirmation)
  - After ejecting, returns to waiting screen and polls for reconnection
- New `device.go` module with platform-aware eject (diskutil on macOS, umount on Linux)

### Changed
- App no longer exits with an error when the device is not connected
- CLI args are now optional; auto-detection is used when no args are provided

## [0.2.2] - 2026-03-04

### Added
- Playlist backup feature: press `b` on playlist picker to backup all playlists
  - Saves all .m3u/.m3u8 files and podcast config to ~/.rockbox-playlist/backups/
  - Transactional writes (atomic rename, cleanup on failure)
  - Auto-prunes old backups (keeps last 10)
  - Toast popup confirms backup with path

## [0.2.1] - 2026-03-04

### Changed
- After adding songs to a playlist, return to playlist picker instead of quitting
- Show a centered toast popup confirming the addition (auto-dismisses after 2s)
- Tighter spacing in playlist picker and podcast menu screens

### Fixed
- 403 errors when fetching podcast RSS feeds (added proper User-Agent header)

## [0.2.0] - 2026-03-02

### Added
- Podcast management feature for playlists named "podcasts"
  - Update all subscribed podcasts (fetch new episodes, keep latest 3, delete old)
  - Search and add new podcasts via iTunes API
  - Download episodes automatically
  - Automatic playlist regeneration sorted by date
  - Detailed progress output during updates and additions
  - Browse & add songs option for manual audio file additions

## [0.1.0] - 2026-03-02

### Added
- Initial release
- TUI playlist picker with filtering
- Song browser with type-to-search filtering
- Multi-select songs with tab key
- Selected songs panel showing picks in selection order
- Confirmation screen before adding songs
- User-friendly error messages with styled display
- Support for custom playlist and music directory paths via CLI arguments

### Fixed
- Space key now inputs into search instead of toggling selection
