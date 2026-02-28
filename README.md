# rockbox-playlist

A terminal app for managing playlists on a Rockbox player. Select a playlist, search and pick songs, and append them to m3u8 playlist files.

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

## Acknowledgements

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
