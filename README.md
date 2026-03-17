# rockbox-playlist

A terminal app for managing playlists on a Rockbox player. Select a playlist, search and pick songs, and append them to m3u8 playlist files.

## Demo

### Playlist Management

![Playlist demo](assets/demo.gif)

### Podcast Management

Select a playlist named "podcasts" to access podcast features: update subscriptions, search and add new podcasts via iTunes, or browse audio files manually.

![Podcast demo](assets/demo-podcast.gif)

## Build

Requires Go. With Nix:

```bash
nix develop
go build -o rockbox-playlist .
```

Or directly:

```bash
go build -o rockbox-playlist .
```

## Configuration

Configuration is loaded from `~/.config/rockbox-playlist/config.toml` (or `$XDG_CONFIG_HOME/rockbox-playlist/config.toml`). All settings have sensible defaults and the file is optional.

Generate a default config file:

```bash
mkdir -p ~/.config/rockbox-playlist
rockbox-playlist config > ~/.config/rockbox-playlist/config.toml
```

Settings can also be overridden with CLI flags or environment variables. Precedence (highest to lowest):

1. CLI flags (`--device-path`, `--sync-source`, `--host`, `--port`, `--host-key-dir`)
2. Environment variables (`ROCKBOX_DEVICE_PATH`, `ROCKBOX_SYNC_SOURCE`, `ROCKBOX_SSH_PORT`)
3. Config file
4. Built-in defaults

See [`config.toml`](config.toml) for all available options and their defaults.

## Usage

```bash
# Uses default paths (/Volumes/NO NAME/Playlists, /Volumes/NO NAME/Music)
./rockbox-playlist

# Custom paths
./rockbox-playlist /path/to/Playlists /path/to/Music
```

### Controls

| Key     | Action                    |
|---------|---------------------------|
| `/`     | Filter playlists          |
| Enter   | Select playlist / confirm |
| Tab     | Toggle song selection     |
| Esc     | Clear search / go back    |
| y / n   | Confirm / cancel adding   |
| q       | Quit                      |

### Podcast Feature

When you select a playlist named "podcasts", a special podcast management menu appears:

1. **Update all podcasts** - Fetch new episodes for all subscribed podcasts (keeps latest 3, deletes old)
2. **Add new podcast** - Search iTunes and subscribe to a new podcast
3. **Browse & add songs** - Manually add audio files to the playlist
4. **Back** - Return to playlist picker

Podcast episodes are stored in `Audiobooks/<podcast-name>/` and configuration in `Audiobooks/podcasts.json`.

## Acknowledgements

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
