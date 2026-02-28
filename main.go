package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen states
type screen int

const (
	screenLoading screen = iota
	screenPlaylistPicker
	screenSongBrowser
	screenConfirmation
	screenDone
)

// Custom key bindings
type keyMap struct {
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
	Yes    key.Binding
	No     key.Binding
}

var keys = keyMap{
	Select: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Yes: key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "confirm"),
	),
	No: key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n", "cancel"),
	),
}

// Styles
var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Padding(0, 1)
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	normalItemStyle   = lipgloss.NewStyle()
	statusBarStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	confirmTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(1, 0)
	songListStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 2)
	loadingStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(2, 4)
	searchStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// minimalDelegate is a lightweight item delegate for better performance
type minimalDelegate struct{}

func (d minimalDelegate) Height() int                             { return 1 }
func (d minimalDelegate) Spacing() int                            { return 0 }
func (d minimalDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d minimalDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(songItem)
	if !ok {
		return
	}

	str := i.Title()

	if index == m.Index() {
		str = selectedItemStyle.Render("> " + str)
	} else {
		str = normalItemStyle.Render("  " + str)
	}

	fmt.Fprint(w, str)
}

// playlistItem implements list.Item for playlists
type playlistItem struct {
	playlist Playlist
}

func (i playlistItem) Title() string       { return i.playlist.Name }
func (i playlistItem) Description() string { return "" }
func (i playlistItem) FilterValue() string { return i.playlist.Name }

// songItem implements list.Item for songs
type songItem struct {
	song     Song
	selected bool
}

func (i songItem) Title() string {
	prefix := "  "
	if i.selected {
		prefix = "✓ "
	}
	return prefix + i.song.DisplayName()
}
func (i songItem) Description() string { return "" }
func (i songItem) FilterValue() string { return i.song.DisplayName() }

// Model is the main application state
type Model struct {
	screen           screen
	playlistDir      string
	musicDir         string
	playlists        []Playlist
	songs            []Song
	songsLoaded      bool
	playlistsLoaded  bool
	selectedPlaylist *Playlist
	selectedSongs    map[string]bool
	playlistList     list.Model
	songList         list.Model
	searchInput      textinput.Model
	filteredIndices  []int // Indices into songList.Items() that match search
	existingEntries  map[string]bool
	width            int
	height           int
	err              error
	message          string
}

// Messages
type songsLoadedMsg struct {
	songs []Song
}

type playlistsLoadedMsg struct {
	playlists []Playlist
}

type errMsg struct {
	err error
}

func initialModel(playlistDir, musicDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 100
	ti.Width = 50

	return Model{
		screen:          screenLoading,
		playlistDir:     playlistDir,
		musicDir:        musicDir,
		selectedSongs:   make(map[string]bool),
		existingEntries: make(map[string]bool),
		searchInput:     ti,
		width:           80,
		height:          24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadPlaylists(m.playlistDir),
		loadSongs(m.musicDir),
	)
}

func loadPlaylists(dir string) tea.Cmd {
	return func() tea.Msg {
		playlists, err := LoadPlaylists(dir)
		if err != nil {
			return errMsg{err}
		}
		return playlistsLoadedMsg{playlists}
	}
}

func loadSongs(dir string) tea.Cmd {
	return func() tea.Msg {
		songs, err := ScanMusicDirectory(dir)
		if err != nil {
			return errMsg{err}
		}
		return songsLoadedMsg{songs}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.playlistList.Items() != nil {
			m.playlistList.SetSize(msg.Width, msg.Height-4)
		}
		if m.songList.Items() != nil {
			m.songList.SetSize(msg.Width, msg.Height-8)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case playlistsLoadedMsg:
		m.playlists = msg.playlists
		m.playlistsLoaded = true
		m.setupPlaylistList()
		if m.songsLoaded && m.playlistsLoaded {
			m.screen = screenPlaylistPicker
		}
		return m, nil

	case songsLoadedMsg:
		m.songs = msg.songs
		m.songsLoaded = true
		if m.songsLoaded && m.playlistsLoaded {
			m.screen = screenPlaylistPicker
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		switch m.screen {
		case screenLoading:
			return m, nil
		case screenPlaylistPicker:
			return m.updatePlaylistPicker(msg)
		case screenSongBrowser:
			return m.updateSongBrowser(msg)
		case screenConfirmation:
			return m.updateConfirmation(msg)
		case screenDone:
			return m, tea.Quit
		}
	}

	// Update the appropriate list
	var cmd tea.Cmd
	switch m.screen {
	case screenPlaylistPicker:
		m.playlistList, cmd = m.playlistList.Update(msg)
	case screenSongBrowser:
		m.songList, cmd = m.songList.Update(msg)
	}
	return m, cmd
}

func (m *Model) setupPlaylistList() {
	items := make([]list.Item, len(m.playlists))
	for i, p := range m.playlists {
		items[i] = playlistItem{playlist: p}
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	m.playlistList = list.New(items, delegate, m.width, m.height-4)
	m.playlistList.Title = "Select a Playlist"
	m.playlistList.SetShowStatusBar(true)
	m.playlistList.SetFilteringEnabled(true)
	m.playlistList.Styles.Title = titleStyle
}

func (m Model) updatePlaylistPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case msg.Type == tea.KeyEnter:
		if item, ok := m.playlistList.SelectedItem().(playlistItem); ok {
			m.selectedPlaylist = &item.playlist
			// Load existing entries to filter duplicates
			entries, _ := LoadPlaylistEntries(item.playlist.Path)
			m.existingEntries = make(map[string]bool)
			for _, e := range entries {
				normalized := NormalizePath(e)
				m.existingEntries[normalized] = true
			}
			// Setup song list
			m.setupSongList()
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.screen = screenSongBrowser
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.playlistList, cmd = m.playlistList.Update(msg)
	return m, cmd
}

func (m *Model) setupSongList() {
	// Filter out songs already in the playlist
	var items []list.Item
	for _, song := range m.songs {
		relPath := song.RelativePath(m.musicDir)
		normalized := NormalizePath(relPath)
		if m.existingEntries[normalized] {
			continue
		}
		items = append(items, songItem{song: song, selected: m.selectedSongs[song.Path]})
	}

	// Use minimal delegate for performance with large lists
	m.songList = list.New(items, minimalDelegate{}, m.width, m.height-8)
	m.songList.Title = ""
	m.songList.SetShowStatusBar(false)
	m.songList.SetFilteringEnabled(false) // We handle filtering ourselves
	m.songList.Styles.Title = titleStyle
	m.songList.SetShowHelp(false)
}

func (m Model) updateSongBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		if m.searchInput.Value() != "" {
			// Clear search first
			m.searchInput.SetValue("")
			m.filterSongs()
			return m, nil
		}
		// Go back to playlist picker
		m.screen = screenPlaylistPicker
		m.selectedSongs = make(map[string]bool)
		return m, nil
	case tea.KeyEnter:
		if len(m.selectedSongs) > 0 {
			m.screen = screenConfirmation
		}
		return m, nil
	case tea.KeyUp:
		m.songList, _ = m.songList.Update(msg)
		return m, nil
	case tea.KeyDown:
		m.songList, _ = m.songList.Update(msg)
		return m, nil
	case tea.KeyTab:
		// Toggle selection on current item
		if item, ok := m.songList.SelectedItem().(songItem); ok {
			if m.selectedSongs[item.song.Path] {
				delete(m.selectedSongs, item.song.Path)
			} else {
				m.selectedSongs[item.song.Path] = true
			}
			m.refreshSongList()
		}
		return m, nil
	case tea.KeyBackspace:
		// Handle backspace in search
		m.searchInput, _ = m.searchInput.Update(msg)
		m.filterSongs()
		return m, nil
	case tea.KeySpace, tea.KeyRunes:
		// Handle typing in search (space is its own key type, not a rune)
		m.searchInput, _ = m.searchInput.Update(msg)
		m.filterSongs()
		return m, nil
	}

	return m, nil
}

func (m *Model) filterSongs() {
	query := strings.ToLower(m.searchInput.Value())
	
	// Pre-allocate with estimated capacity
	items := make([]list.Item, 0, len(m.songs)/2)

	for _, song := range m.songs {
		relPath := song.RelativePath(m.musicDir)
		normalized := NormalizePath(relPath)
		if m.existingEntries[normalized] {
			continue
		}

		// Filter by search query using cached lowercase name
		if query != "" && !strings.Contains(song.SearchName(), query) {
			continue
		}

		items = append(items, songItem{song: song, selected: m.selectedSongs[song.Path]})
	}

	m.songList.SetItems(items)
}

func (m *Model) refreshSongList() {
	items := m.songList.Items()
	for i, item := range items {
		if si, ok := item.(songItem); ok {
			si.selected = m.selectedSongs[si.song.Path]
			items[i] = si
		}
	}
	m.songList.SetItems(items)
}

func (m Model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Yes):
		// Add songs to playlist
		var entries []string
		for _, song := range m.songs {
			if m.selectedSongs[song.Path] {
				entries = append(entries, song.RelativePath(m.musicDir))
			}
		}
		err := AppendToPlaylist(m.selectedPlaylist.Path, entries)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.message = fmt.Sprintf("Added %d songs to %s", len(entries), m.selectedPlaylist.Name)
		m.screen = screenDone
		return m, tea.Quit
	case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
		m.screen = screenSongBrowser
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress any key to exit.", m.err)
	}

	switch m.screen {
	case screenLoading:
		return m.viewLoading()
	case screenPlaylistPicker:
		return m.viewPlaylistPicker()
	case screenSongBrowser:
		return m.viewSongBrowser()
	case screenConfirmation:
		return m.viewConfirmation()
	case screenDone:
		return m.message + "\n"
	}
	return ""
}

func (m Model) viewLoading() string {
	var status strings.Builder
	status.WriteString(loadingStyle.Render("Loading..."))
	status.WriteString("\n\n")

	if m.playlistsLoaded {
		status.WriteString("  ✓ Playlists loaded\n")
	} else {
		status.WriteString("  ⋯ Loading playlists...\n")
	}

	if m.songsLoaded {
		status.WriteString(fmt.Sprintf("  ✓ %d songs loaded\n", len(m.songs)))
	} else {
		status.WriteString("  ⋯ Scanning music library...\n")
	}

	return status.String()
}

func (m Model) viewPlaylistPicker() string {
	return m.playlistList.View()
}

func (m Model) viewSongBrowser() string {
	var b strings.Builder

	// Header with playlist name
	header := headerStyle.Render(fmt.Sprintf("Adding to: %s", m.selectedPlaylist.Name))
	b.WriteString(header)
	b.WriteString("\n")

	// Search box
	searchBox := searchStyle.Render("Search: ") + m.searchInput.Value()
	if m.searchInput.Value() == "" {
		searchBox = searchStyle.Render("Search: (type to filter)")
	}
	b.WriteString(searchBox)
	b.WriteString("\n")

	// Status
	selectedCount := len(m.selectedSongs)
	status := statusBarStyle.Render(fmt.Sprintf("%d selected | tab: select | enter: confirm | esc: back/clear", selectedCount))
	b.WriteString(status)
	b.WriteString("\n\n")

	// Song list
	b.WriteString(m.songList.View())

	return b.String()
}

func (m Model) viewConfirmation() string {
	var b strings.Builder

	b.WriteString(confirmTitleStyle.Render(fmt.Sprintf("Add %d songs to \"%s\"?", len(m.selectedSongs), m.selectedPlaylist.Name)))
	b.WriteString("\n\n")

	// List selected songs
	for _, song := range m.songs {
		if m.selectedSongs[song.Path] {
			b.WriteString(songListStyle.Render("• " + song.ConfirmDisplayName()))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("Press y to confirm, n to cancel"))

	return b.String()
}

func main() {
	// Default paths - can be overridden with args
	playlistDir := "/Volumes/NO NAME/Playlists"
	musicDir := "/Volumes/NO NAME/Music"

	if len(os.Args) >= 3 {
		playlistDir = os.Args[1]
		musicDir = os.Args[2]
	}

	// Validate directories exist
	if _, err := os.Stat(playlistDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Playlist directory not found: %s\n", playlistDir)
		os.Exit(1)
	}
	if _, err := os.Stat(musicDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Music directory not found: %s\n", musicDir)
		os.Exit(1)
	}

	// Ensure paths are absolute
	playlistDir, _ = filepath.Abs(playlistDir)
	musicDir, _ = filepath.Abs(musicDir)

	p := tea.NewProgram(initialModel(playlistDir, musicDir), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print final message if any
	if m, ok := finalModel.(Model); ok && m.message != "" {
		fmt.Println(m.message)
	}
}
