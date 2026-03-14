package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen states
type screen int

const (
	screenNoDevice screen = iota
	screenLoading
	screenPlaylistPicker
	screenSongBrowser
	screenConfirmation
	screenDone
	// Sync screens
	screenSyncConfirm
	screenSync
	// Podcast screens
	screenPodcastMenu
	screenPodcastUpdate
	screenPodcastSearch
	screenPodcastResults
	screenPodcastAdding
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
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	headerStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Padding(0, 1)
	selectedItemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	normalItemStyle    = lipgloss.NewStyle()
	statusBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	confirmTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(1, 0)
	songListStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 2)
	loadingStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(2, 4)
	searchStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderTop(false).
				BorderBottom(false).
				BorderRight(false).
				BorderForeground(lipgloss.Color("241")).
				PaddingLeft(1)
	selectedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	selectedDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	selectedSongStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errorTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Padding(1, 2)
	errorMsgStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 2)
	errorHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(1, 2)
	toastStyle         = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("35")).
				Padding(1, 3)
	// Device styles
	waitingStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(2, 4)
	waitingHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 4)
	ejectConfirmStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("166")).
				Padding(1, 3)
	// Sync styles
	syncTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	syncOutputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	syncConfirmStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("33")).
				Padding(1, 3)
	syncRetryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("166")).
			Padding(1, 3)
	// Podcast styles
	podcastWrapStyle     = lipgloss.NewStyle().Padding(1, 2)
	podcastTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	podcastItemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	podcastSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	podcastProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	podcastResultStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	podcastArtistStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
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
	appConfig        AppConfig
	screen           screen
	playlistDir      string
	musicDir         string
	playlists        []Playlist
	songs            []Song
	songsLoaded      bool
	playlistsLoaded  bool
	selectedPlaylist *Playlist
	selectedSongs    map[string]bool
	selectedOrder    []string // Song paths in selection order
	playlistList     list.Model
	songList         list.Model
	searchInput      textinput.Model
	filteredIndices  []int // Indices into songList.Items() that match search
	existingEntries  map[string]bool
	width            int
	height           int
	err              error
	message          string

	// Device fields
	deviceConnected bool
	devicePath      string // Base mount path, e.g. /Volumes/NO NAME
	confirmEject    bool   // Whether eject confirmation overlay is showing

	// Sync fields
	syncSource     string // Source directory for music sync
	syncInProgress bool   // Whether sync is running
	syncOutput     string // Rsync output

	// Podcast fields
	podcastConfig      PodcastConfig
	podcastAudioDir    string // e.g. /Volumes/NO NAME/Audiobooks
	podcastConfigPath  string // e.g. /Volumes/NO NAME/Audiobooks/podcasts.json
	podcastMenuIndex   int    // Selected menu item (0-3)
	podcastSearchInput textinput.Model
	podcastResults     []iTunesPodcast
	podcastResultIndex int
	podcastProgress    string   // Current progress message
	podcastLog         []string // Log of progress messages
	podcastUpdating    bool     // Whether update is in progress
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

type toastDismissMsg struct{}

type musicRescanMsg struct{}

type backupDoneMsg struct {
	err     error
	summary string
}

// Podcast messages
type podcastUpdateProgressMsg struct{ message string }
type podcastUpdateDoneMsg struct {
	err     error
	summary string
}
type podcastSearchResultsMsg struct {
	results []iTunesPodcast
	err     error
}
type podcastAddProgressMsg struct{ message string }
type podcastAddDoneMsg struct {
	err     error
	summary string
}

func initialModel(cfg AppConfig, devicePath, playlistDir, musicDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 100
	ti.Width = 50

	// Podcast search input
	podcastTi := textinput.New()
	podcastTi.Placeholder = "Search podcasts..."
	podcastTi.CharLimit = 100
	podcastTi.Width = 40

	// Determine initial screen based on device presence
	startScreen := screenNoDevice
	deviceConnected := false
	var podcastAudioDir, podcastConfigPath string

	if devicePath != "" && CheckDevice(devicePath, cfg.Device.MusicDir, cfg.Device.PlaylistDir) {
		startScreen = screenLoading
		deviceConnected = true
		podcastAudioDir = filepath.Join(devicePath, cfg.Device.AudiobooksDir)
		podcastConfigPath = filepath.Join(podcastAudioDir, "podcasts.json")
	}

	return Model{
		appConfig:          cfg,
		screen:             startScreen,
		deviceConnected:    deviceConnected,
		devicePath:         devicePath,
		playlistDir:        playlistDir,
		musicDir:           musicDir,
		syncSource:         cfg.Sync.Source,
		selectedSongs:      make(map[string]bool),
		existingEntries:    make(map[string]bool),
		searchInput:        ti,
		width:              80,
		height:             24,
		podcastSearchInput: podcastTi,
		podcastAudioDir:    podcastAudioDir,
		podcastConfigPath:  podcastConfigPath,
		podcastConfig:      make(PodcastConfig),
	}
}

// initialModelServe creates a Model for SSH serve mode.
// Starts on the no-device screen if device is not connected, otherwise loads immediately.
func initialModelServe(cfg AppConfig, devicePath string, width, height int) Model {
	playlistDir := ""
	musicDir := ""
	if devicePath != "" && CheckDevice(devicePath, cfg.Device.MusicDir, cfg.Device.PlaylistDir) {
		playlistDir = filepath.Join(devicePath, cfg.Device.PlaylistDir)
		musicDir = filepath.Join(devicePath, cfg.Device.MusicDir)
	} else if devicePath == "" {
		// Auto-detect
		found := FindDevicePath(cfg.Device.SearchPaths, cfg.Device.MusicDir, cfg.Device.PlaylistDir)
		if found != "" {
			devicePath = found
			playlistDir = filepath.Join(found, cfg.Device.PlaylistDir)
			musicDir = filepath.Join(found, cfg.Device.MusicDir)
		}
	}

	m := initialModel(cfg, devicePath, playlistDir, musicDir)
	m.width = width
	m.height = height
	return m
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenNoDevice {
		return WatchForDevice(m.devicePath, m.appConfig.Device.SearchPaths)
	}
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

func scheduleMusicRescan(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return musicRescanMsg{}
	})
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
			songListWidth := msg.Width - msg.Width*35/100 - 1
			m.songList.SetSize(songListWidth, msg.Height-8)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case toastDismissMsg:
		m.message = ""
		return m, nil

	case backupDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Backup failed: %v", msg.err)
		} else {
			m.message = msg.summary
		}
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return toastDismissMsg{}
		})

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
		wasLoaded := m.songsLoaded
		m.songsLoaded = true
		if m.songsLoaded && m.playlistsLoaded && !wasLoaded {
			// Initial load complete — show playlist picker
			m.screen = screenPlaylistPicker
		}
		if wasLoaded && m.screen == screenSongBrowser {
			// Re-scan while browsing — refresh the song list
			m.filterSongs()
		}
		// Schedule the next background re-scan
		return m, scheduleMusicRescan(m.appConfig.RescanDuration())

	case musicRescanMsg:
		// Only re-scan if device is connected
		if m.deviceConnected && m.musicDir != "" {
			return m, loadSongs(m.musicDir)
		}
		return m, nil

	case podcastUpdateDoneMsg:
		m.podcastUpdating = false
		if msg.err != nil {
			m.podcastProgress = fmt.Sprintf("Error: %v\n\n%s", msg.err, msg.summary)
		} else {
			m.podcastProgress = msg.summary
		}
		// Stay on update screen to show results, user presses key to continue
		return m, nil

	case podcastSearchResultsMsg:
		if msg.err != nil {
			m.podcastProgress = fmt.Sprintf("Error: %v", msg.err)
			return m, nil
		}
		m.podcastResults = msg.results
		m.podcastResultIndex = 0
		m.screen = screenPodcastResults
		return m, nil

	case podcastAddDoneMsg:
		m.podcastUpdating = false
		if msg.err != nil {
			m.podcastProgress = fmt.Sprintf("Error: %v\n\n%s", msg.err, msg.summary)
		} else {
			m.podcastProgress = msg.summary
			// Reload config
			config, err := LoadPodcastConfig(m.podcastConfigPath)
			if err == nil {
				m.podcastConfig = config
			}
		}
		// Stay on adding screen to show results
		return m, nil

	case syncDoneMsg:
		m.syncInProgress = false
		if msg.err != nil {
			m.syncOutput = fmt.Sprintf("Error: %v\n\n%s", msg.err, msg.output)
		} else {
			m.syncOutput = msg.output
		}
		// Stay on sync screen to show results
		return m, nil

	case deviceStatusMsg:
		if msg.connected {
			m.deviceConnected = true
			m.devicePath = msg.path
			m.playlistDir = filepath.Join(msg.path, m.appConfig.Device.PlaylistDir)
			m.musicDir = filepath.Join(msg.path, m.appConfig.Device.MusicDir)
			m.podcastAudioDir = filepath.Join(msg.path, m.appConfig.Device.AudiobooksDir)
			m.podcastConfigPath = filepath.Join(m.podcastAudioDir, "podcasts.json")
			m.screen = screenLoading
			m.songsLoaded = false
			m.playlistsLoaded = false
			return m, tea.Batch(
				loadPlaylists(m.playlistDir),
				loadSongs(m.musicDir),
			)
		}
		// Device not found yet, keep polling
		return m, WatchForDevice(m.devicePath, m.appConfig.Device.SearchPaths)

	case deviceEjectMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Eject failed: %v", msg.err)
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return toastDismissMsg{}
			})
		}
		// Eject succeeded — reset state and go to no-device screen
		m.deviceConnected = false
		m.confirmEject = false
		m.playlists = nil
		m.songs = nil
		m.songsLoaded = false
		m.playlistsLoaded = false
		m.selectedPlaylist = nil
		m.selectedSongs = make(map[string]bool)
		m.selectedOrder = nil
		m.screen = screenNoDevice
		m.message = ""
		return m, WatchForDevice(m.devicePath, m.appConfig.Device.SearchPaths)

	case tea.KeyMsg:
		// Global quit
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		switch m.screen {
		case screenNoDevice:
			if key.Matches(msg, keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		case screenLoading:
			return m, nil
		case screenPlaylistPicker:
			return m.updatePlaylistPicker(msg)
		case screenSongBrowser:
			return m.updateSongBrowser(msg)
		case screenConfirmation:
			return m.updateConfirmation(msg)
		case screenSyncConfirm:
			return m.updateSyncConfirm(msg)
		case screenSync:
			return m.updateSync(msg)
		case screenPodcastMenu:
			return m.updatePodcastMenu(msg)
		case screenPodcastUpdate:
			return m.updatePodcastUpdate(msg)
		case screenPodcastSearch:
			return m.updatePodcastSearch(msg)
		case screenPodcastResults:
			return m.updatePodcastResults(msg)
		case screenPodcastAdding:
			return m.updatePodcastAdding(msg)
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
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	m.playlistList = list.New(items, delegate, m.width, m.height-4)
	m.playlistList.Title = "Select a Playlist"
	m.playlistList.SetShowStatusBar(true)
	m.playlistList.SetFilteringEnabled(true)
	m.playlistList.Styles.Title = titleStyle
	syncSource := m.syncSource
	m.playlistList.AdditionalShortHelpKeys = func() []key.Binding {
		bindings := []key.Binding{
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "backup")),
			key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "eject")),
		}
		if syncSource != "" {
			bindings = append(bindings, key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")))
		}
		return bindings
	}
}

func (m Model) startBackup() tea.Cmd {
	playlistDir := m.playlistDir
	podcastConfigPath := m.podcastConfigPath
	maxBackups := m.appConfig.Backup.MaxBackups
	return func() tea.Msg {
		summary, err := BackupPlaylists(playlistDir, podcastConfigPath, maxBackups)
		return backupDoneMsg{err: err, summary: summary}
	}
}

func (m Model) updatePlaylistPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle eject confirmation overlay
	if m.confirmEject {
		switch {
		case key.Matches(msg, keys.Yes):
			m.confirmEject = false
			return m, EjectDevice(m.devicePath)
		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.confirmEject = false
			return m, nil
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case msg.String() == "u" && m.playlistList.FilterState() == list.Unfiltered:
		m.confirmEject = true
		return m, nil
	case msg.String() == "s" && m.playlistList.FilterState() == list.Unfiltered:
		return m.handleSyncKey()
	case msg.String() == "b" && m.playlistList.FilterState() == list.Unfiltered:
		return m, m.startBackup()
	case msg.Type == tea.KeyEnter:
		if item, ok := m.playlistList.SelectedItem().(playlistItem); ok {
			m.selectedPlaylist = &item.playlist
			m.message = "" // Clear any previous success message

			// Check if this is a podcast playlist
			if IsPodcastPlaylist(item.playlist.Name) {
				// Load podcast config
				config, err := LoadPodcastConfig(m.podcastConfigPath)
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.podcastConfig = config
				m.podcastMenuIndex = 0
				m.screen = screenPodcastMenu
				return m, nil
			}

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
	songListWidth := m.width - m.width*35/100 - 1
	m.songList = list.New(items, minimalDelegate{}, songListWidth, m.height-8)
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
		m.selectedOrder = nil
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
				// Remove from selectedOrder
				for i, p := range m.selectedOrder {
					if p == item.song.Path {
						m.selectedOrder = append(m.selectedOrder[:i], m.selectedOrder[i+1:]...)
						break
					}
				}
			} else {
				m.selectedSongs[item.song.Path] = true
				m.selectedOrder = append(m.selectedOrder, item.song.Path)
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
		// Reset selection and go back to playlist picker with toast
		m.message = fmt.Sprintf("Added %d songs to %s", len(entries), m.selectedPlaylist.Name)
		m.selectedSongs = make(map[string]bool)
		m.selectedOrder = nil
		m.selectedPlaylist = nil
		m.screen = screenPlaylistPicker
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return toastDismissMsg{}
		})
	case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
		m.screen = screenSongBrowser
		return m, nil
	}
	return m, nil
}

// handleSyncKey handles pressing 's' on the playlist picker.
func (m Model) handleSyncKey() (tea.Model, tea.Cmd) {
	if m.syncSource == "" {
		m.message = "No sync source configured. Use --sync-source flag."
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return toastDismissMsg{}
		})
	}

	// Check if source directory exists
	if _, err := os.Stat(m.syncSource); os.IsNotExist(err) {
		// Source not mounted — show retry prompt
		m.syncOutput = fmt.Sprintf("Source not found: %s", m.syncSource)
		m.screen = screenSyncConfirm
		return m, nil
	}

	// Source exists — show sync confirmation
	m.syncOutput = ""
	m.screen = screenSyncConfirm
	return m, nil
}

func (m Model) updateSyncConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Yes):
		// Check source exists (might be a retry)
		if _, err := os.Stat(m.syncSource); os.IsNotExist(err) {
			// Still not found
			m.syncOutput = fmt.Sprintf("Source not found: %s", m.syncSource)
			return m, nil
		}
		// Start sync
		m.syncInProgress = true
		m.syncOutput = ""
		m.screen = screenSync
		return m, runSync(m.syncSource, m.musicDir)
	case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
		m.screen = screenPlaylistPicker
		return m, nil
	}
	return m, nil
}

func (m Model) updateSync(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyEnter:
		// Only allow exit if sync is complete
		if !m.syncInProgress {
			m.screen = screenPlaylistPicker
			m.syncOutput = ""
		}
		return m, nil
	}
	return m, nil
}

// Podcast menu items
var podcastMenuItems = []string{
	"Update all podcasts",
	"Add new podcast",
	"Browse & add songs",
	"Back",
}

func (m Model) updatePodcastMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyUp:
		if m.podcastMenuIndex > 0 {
			m.podcastMenuIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.podcastMenuIndex < len(podcastMenuItems)-1 {
			m.podcastMenuIndex++
		}
		return m, nil
	case tea.KeyEnter:
		switch m.podcastMenuIndex {
		case 0: // Update all podcasts
			m.screen = screenPodcastUpdate
			m.podcastProgress = ""
			m.podcastUpdating = true
			return m, m.startPodcastUpdate()
		case 1: // Add new podcast
			m.screen = screenPodcastSearch
			m.podcastSearchInput.SetValue("")
			m.podcastSearchInput.Focus()
			m.podcastResults = nil
			return m, nil
		case 2: // Browse & add songs
			// Load existing entries to filter duplicates
			entries, _ := LoadPlaylistEntries(m.selectedPlaylist.Path)
			m.existingEntries = make(map[string]bool)
			for _, e := range entries {
				normalized := NormalizePath(e)
				m.existingEntries[normalized] = true
			}
			m.setupSongList()
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.screen = screenSongBrowser
			return m, nil
		case 3: // Back
			m.screen = screenPlaylistPicker
			return m, nil
		}
	case tea.KeyEsc:
		m.screen = screenPlaylistPicker
		return m, nil
	}
	return m, nil
}

func (m Model) startPodcastUpdate() tea.Cmd {
	config := m.podcastConfig
	audioDir := m.podcastAudioDir
	configPath := m.podcastConfigPath
	playlistPath := m.selectedPlaylist.Path
	episodesToKeep := m.appConfig.Podcast.EpisodesToKeep

	return func() tea.Msg {
		var log []string
		totalNew := 0
		totalDeleted := 0

		if len(config) == 0 {
			return podcastUpdateDoneMsg{err: nil, summary: "No podcasts subscribed"}
		}

		for name := range config {
			feed := config[name]
			log = append(log, fmt.Sprintf("Checking %s...", name))

			newCount, deletedCount, msgs, err := UpdatePodcastWithLog(name, &feed, audioDir, episodesToKeep)
			log = append(log, msgs...)

			if err != nil {
				log = append(log, fmt.Sprintf("  Error: %v", err))
				continue
			}

			config[name] = feed
			totalNew += newCount
			totalDeleted += deletedCount

			if newCount == 0 {
				log = append(log, "  No new episodes")
			} else if newCount == 1 {
				log = append(log, "  1 new episode downloaded")
			} else {
				log = append(log, fmt.Sprintf("  %d new episodes downloaded", newCount))
			}
		}

		// Save config
		if err := SavePodcastConfig(configPath, config); err != nil {
			return podcastUpdateDoneMsg{err: err, summary: strings.Join(log, "\n")}
		}

		// Rebuild playlist
		if err := RebuildPodcastPlaylist(config, playlistPath, audioDir); err != nil {
			return podcastUpdateDoneMsg{err: err, summary: strings.Join(log, "\n")}
		}

		log = append(log, "")
		log = append(log, "Done!")

		summary := fmt.Sprintf("Downloaded %d new episodes", totalNew)
		if totalDeleted > 0 {
			summary += fmt.Sprintf(", deleted %d old", totalDeleted)
		}

		return podcastUpdateDoneMsg{err: nil, summary: strings.Join(log, "\n")}
	}
}

func (m Model) updatePodcastUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyEnter:
		// Only allow exit if update is complete
		if !m.podcastUpdating {
			m.screen = screenPodcastMenu
			m.podcastProgress = ""
			// Reload config since it may have been updated
			config, err := LoadPodcastConfig(m.podcastConfigPath)
			if err == nil {
				m.podcastConfig = config
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updatePodcastSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenPodcastMenu
		return m, nil
	case tea.KeyEnter:
		query := m.podcastSearchInput.Value()
		if query != "" {
			m.podcastProgress = "Searching..."
			return m, m.searchPodcasts(query)
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.podcastSearchInput, cmd = m.podcastSearchInput.Update(msg)
		return m, cmd
	}
}

func (m Model) searchPodcasts(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := SearchPodcasts(query)
		return podcastSearchResultsMsg{results: results, err: err}
	}
}

func (m Model) updatePodcastResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenPodcastSearch
		return m, nil
	case tea.KeyUp:
		if m.podcastResultIndex > 0 {
			m.podcastResultIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.podcastResultIndex < len(m.podcastResults)-1 {
			m.podcastResultIndex++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.podcastResults) > 0 {
			m.screen = screenPodcastAdding
			m.podcastProgress = ""
			m.podcastUpdating = true
			return m, m.addSelectedPodcast()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) addSelectedPodcast() tea.Cmd {
	podcast := m.podcastResults[m.podcastResultIndex]
	config := m.podcastConfig
	audioDir := m.podcastAudioDir
	configPath := m.podcastConfigPath
	playlistPath := m.selectedPlaylist.Path
	episodesToKeep := m.appConfig.Podcast.EpisodesToKeep

	return func() tea.Msg {
		log, err := AddPodcastWithLog(podcast, audioDir, config, episodesToKeep)

		if err != nil {
			return podcastAddDoneMsg{err: err, summary: strings.Join(log, "\n")}
		}

		// Save config
		if err := SavePodcastConfig(configPath, config); err != nil {
			return podcastAddDoneMsg{err: err, summary: strings.Join(log, "\n")}
		}

		// Rebuild playlist
		if err := RebuildPodcastPlaylist(config, playlistPath, audioDir); err != nil {
			return podcastAddDoneMsg{err: err, summary: strings.Join(log, "\n")}
		}

		return podcastAddDoneMsg{err: nil, summary: strings.Join(log, "\n")}
	}
}

func (m Model) updatePodcastAdding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyEnter:
		// Only allow exit if add is complete
		if !m.podcastUpdating {
			m.screen = screenPodcastMenu
			m.podcastProgress = ""
		}
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		var b strings.Builder
		b.WriteString(errorTitleStyle.Render("Something went wrong"))
		b.WriteString("\n")
		b.WriteString(errorMsgStyle.Render(m.err.Error()))
		b.WriteString("\n")
		b.WriteString(errorHintStyle.Render("Press any key to exit."))
		return b.String()
	}

	switch m.screen {
	case screenNoDevice:
		return m.viewNoDevice()
	case screenLoading:
		return m.viewLoading()
	case screenPlaylistPicker:
		return m.viewPlaylistPicker()
	case screenSongBrowser:
		return m.viewSongBrowser()
	case screenConfirmation:
		return m.viewConfirmation()
	case screenSyncConfirm:
		return m.viewSyncConfirm()
	case screenSync:
		return m.viewSync()
	case screenPodcastMenu:
		return m.viewPodcastMenu()
	case screenPodcastUpdate:
		return m.viewPodcastUpdate()
	case screenPodcastSearch:
		return m.viewPodcastSearch()
	case screenPodcastResults:
		return m.viewPodcastResults()
	case screenPodcastAdding:
		return m.viewPodcastAdding()
	case screenDone:
		return m.message + "\n"
	}
	return ""
}

func (m Model) viewNoDevice() string {
	title := waitingStyle.Render("No Rockbox player detected")
	hint := waitingHintStyle.Render("Waiting for device... (q: quit)")
	content := title + "\n\n" + hint
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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
	base := m.playlistList.View()

	if m.confirmEject {
		toast := ejectConfirmStyle.Render("Eject player? (y/n)")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, toast)
	}

	if m.message != "" {
		toast := toastStyle.Render(m.message)
		overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, toast)
		return overlay
	}

	return base
}

func (m Model) viewSongBrowser() string {
	// Calculate panel widths
	rightWidth := m.width * 35 / 100
	if rightWidth < 20 {
		rightWidth = 20
	}
	leftWidth := m.width - rightWidth - 1 // -1 for border

	// === Left panel: song picker ===
	var left strings.Builder

	// Header with playlist name
	header := headerStyle.Render(fmt.Sprintf("Adding to: %s", m.selectedPlaylist.Name))
	left.WriteString(header)
	left.WriteString("\n")

	// Search box
	searchBox := searchStyle.Render("Search: ") + m.searchInput.Value()
	if m.searchInput.Value() == "" {
		searchBox = searchStyle.Render("Search: (type to filter)")
	}
	left.WriteString(searchBox)
	left.WriteString("\n")

	// Status
	selectedCount := len(m.selectedSongs)
	status := statusBarStyle.Render(fmt.Sprintf("%d selected | tab: select | enter: confirm | esc: back/clear", selectedCount))
	left.WriteString(status)
	left.WriteString("\n\n")

	// Song list
	left.WriteString(m.songList.View())

	leftPanel := lipgloss.NewStyle().Width(leftWidth).Render(left.String())

	// === Right panel: selected songs ===
	var right strings.Builder

	right.WriteString(selectedTitleStyle.Render(fmt.Sprintf("Selected (%d)", selectedCount)))
	right.WriteString("\n\n")

	if len(m.selectedOrder) == 0 {
		right.WriteString(selectedDimStyle.Render("No songs selected"))
	} else {
		// Build a path -> song lookup from the full songs list
		songMap := make(map[string]Song, len(m.songs))
		for _, s := range m.songs {
			songMap[s.Path] = s
		}

		// Show selected songs in selection order, capped to visible height
		maxVisible := m.height - 5
		if maxVisible < 1 {
			maxVisible = 1
		}
		for i, path := range m.selectedOrder {
			if i >= maxVisible {
				remaining := len(m.selectedOrder) - maxVisible
				right.WriteString(selectedDimStyle.Render(fmt.Sprintf("... and %d more", remaining)))
				break
			}
			if song, ok := songMap[path]; ok {
				right.WriteString(selectedSongStyle.Render("• " + song.ConfirmDisplayName()))
			}
			right.WriteString("\n")
		}
	}

	rightPanel := selectedPanelStyle.Width(rightWidth).Height(m.height - 1).Render(right.String())

	// Join panels horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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

func (m Model) viewSyncConfirm() string {
	sourceExists := true
	if _, err := os.Stat(m.syncSource); os.IsNotExist(err) {
		sourceExists = false
	}

	if !sourceExists {
		// Source not mounted — retry prompt
		content := syncRetryStyle.Render(fmt.Sprintf("Source not mounted: %s\n\nRetry? (y/n)", m.syncSource))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	// Normal confirmation
	content := syncConfirmStyle.Render(fmt.Sprintf("Sync music from %s? (y/n)", m.syncSource))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewSync() string {
	var b strings.Builder

	b.WriteString(syncTitleStyle.Render("Syncing Music"))
	b.WriteString("\n")

	if m.syncInProgress {
		b.WriteString(syncOutputStyle.Render("Running rsync, please wait..."))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("This may take a while..."))
	} else {
		// Show rsync output, capped to visible height
		if m.syncOutput != "" {
			lines := strings.Split(m.syncOutput, "\n")
			maxLines := m.height - 6
			if maxLines < 1 {
				maxLines = 1
			}
			if len(lines) > maxLines {
				// Show last N lines
				lines = lines[len(lines)-maxLines:]
			}
			b.WriteString(syncOutputStyle.Render(strings.Join(lines, "\n")))
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	}

	return podcastWrapStyle.Render(b.String())
}

func (m Model) viewPodcastMenu() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Podcast Management"))
	b.WriteString("\n")

	for i, item := range podcastMenuItems {
		if i == m.podcastMenuIndex {
			b.WriteString(podcastSelectedStyle.Render("> " + item))
		} else {
			b.WriteString(podcastItemStyle.Render("  " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render(fmt.Sprintf("%d podcasts subscribed | esc: back", len(m.podcastConfig))))

	return podcastWrapStyle.Render(b.String())
}

func (m Model) viewPodcastUpdate() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Updating Podcasts"))
	b.WriteString("\n")

	if m.podcastUpdating {
		b.WriteString(podcastProgressStyle.Render("Updating podcasts, please wait..."))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("This may take a while..."))
	} else {
		b.WriteString(podcastProgressStyle.Render(m.podcastProgress))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	}

	return podcastWrapStyle.Render(b.String())
}

func (m Model) viewPodcastSearch() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Search Podcasts"))
	b.WriteString("\n")
	b.WriteString("Search: " + m.podcastSearchInput.View())
	b.WriteString("\n\n")
	b.WriteString(statusBarStyle.Render("enter: search | esc: back"))

	return podcastWrapStyle.Render(b.String())
}

func (m Model) viewPodcastResults() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Search Results"))
	b.WriteString("\n")

	if len(m.podcastResults) == 0 {
		b.WriteString(podcastItemStyle.Render("No podcasts found"))
	} else {
		for i, podcast := range m.podcastResults {
			if i == m.podcastResultIndex {
				b.WriteString(podcastSelectedStyle.Render("> " + podcast.CollectionName))
			} else {
				b.WriteString(podcastResultStyle.Render("  " + podcast.CollectionName))
			}
			b.WriteString("\n")
			b.WriteString(podcastArtistStyle.Render("    by " + podcast.ArtistName))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("enter: add podcast | esc: back"))

	return podcastWrapStyle.Render(b.String())
}

func (m Model) viewPodcastAdding() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Adding Podcast"))
	b.WriteString("\n")

	if m.podcastUpdating {
		if m.podcastResultIndex < len(m.podcastResults) {
			b.WriteString(podcastItemStyle.Render(m.podcastResults[m.podcastResultIndex].CollectionName))
			b.WriteString("\n\n")
		}
		b.WriteString(podcastProgressStyle.Render("Downloading episodes, please wait..."))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("This may take a while..."))
	} else {
		b.WriteString(podcastProgressStyle.Render(m.podcastProgress))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	}

	return podcastWrapStyle.Render(b.String())
}

func main() {
	// Load config file (defaults if not found)
	cfg := LoadConfig()

	// Flags (defaults come from config)
	serve := flag.Bool("serve", false, "Run as SSH server")
	host := flag.String("host", cfg.Server.Host, "SSH server listen address")
	port := flag.String("port", cfg.Server.Port, "SSH server listen port")
	hostKeyDir := flag.String("host-key-dir", cfg.Server.HostKeyDir, "Directory for SSH host keys")
	deviceFlag := flag.String("device-path", cfg.Device.Path, "Path to Rockbox device (auto-detect if empty)")
	syncSourceFlag := flag.String("sync-source", cfg.Sync.Source, "Source directory for music sync (e.g. /media/user/Player)")

	flag.Parse()

	// Environment variable overrides (only if flag wasn't explicitly set)
	if envPort := os.Getenv("ROCKBOX_SSH_PORT"); envPort != "" && *port == cfg.Server.Port {
		*port = envPort
	}
	if envPath := os.Getenv("ROCKBOX_DEVICE_PATH"); envPath != "" && *deviceFlag == cfg.Device.Path {
		*deviceFlag = envPath
	}
	if envPath := os.Getenv("ROCKBOX_SYNC_SOURCE"); envPath != "" && *syncSourceFlag == cfg.Sync.Source {
		*syncSourceFlag = envPath
	}

	// Apply resolved values back into config
	cfg.Server.Host = *host
	cfg.Server.Port = *port
	cfg.Server.HostKeyDir = *hostKeyDir
	cfg.Device.Path = *deviceFlag
	cfg.Sync.Source = *syncSourceFlag

	if *serve {
		StartServer(ServerConfig{
			Host:       cfg.Server.Host,
			Port:       cfg.Server.Port,
			HostKeyDir: cfg.Server.HostKeyDir,
			DevicePath: cfg.Device.Path,
			SyncSource: cfg.Sync.Source,
			AppCfg:     cfg,
		})
		return
	}

	// --- Local mode ---
	var devicePath, playlistDir, musicDir string

	args := flag.Args()
	if len(args) >= 2 {
		// Explicit paths provided as positional args
		playlistDir, _ = filepath.Abs(args[0])
		musicDir, _ = filepath.Abs(args[1])
		// Derive device path from playlist dir parent
		devicePath = filepath.Dir(playlistDir)
	} else if cfg.Device.Path != "" {
		// From config/flag/env
		devicePath = cfg.Device.Path
		playlistDir = filepath.Join(devicePath, cfg.Device.PlaylistDir)
		musicDir = filepath.Join(devicePath, cfg.Device.MusicDir)
	} else {
		// Auto-detect device
		devicePath = FindDevicePath(cfg.Device.SearchPaths, cfg.Device.MusicDir, cfg.Device.PlaylistDir)
		if devicePath != "" {
			playlistDir = filepath.Join(devicePath, cfg.Device.PlaylistDir)
			musicDir = filepath.Join(devicePath, cfg.Device.MusicDir)
		}
	}

	p := tea.NewProgram(initialModel(cfg, devicePath, playlistDir, musicDir), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure your terminal supports interactive TUI applications.\n")
		os.Exit(1)
	}

	// Print final message if any
	if m, ok := finalModel.(Model); ok && m.message != "" {
		fmt.Println(m.message)
	}
}
