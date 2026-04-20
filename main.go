package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

//go:embed config.toml
var defaultConfigTOML string

// version is set at build time via ldflags
var version = "dev"

// Screen states
type screen int

const (
	screenNoDevice screen = iota
	screenLoading
	screenPlaylistPicker
	screenSongBrowser
	screenConfirmation
	// Playlist creation
	screenCreatePlaylist
	// Sync screens
	screenSync
	// Podcast screens
	screenPodcastMenu
	screenPodcastUpdate
	screenPodcastSearch
	screenPodcastResults
	screenPodcastAdding
	screenPodcastRemove
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
	addedSongStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("114")) // green for + lines
	errorTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Padding(1, 2)
	errorMsgStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 2)
	errorHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(1, 2)
	toastStyle         = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("35")).
				Padding(1, 3)
	// Device styles
	waitingStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(2, 4)
	waitingHintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 4)
	dangerConfirmStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("166")).
				Padding(1, 3)
	// Sync styles
	syncTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	syncOutputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	// Podcast styles
	wrapStyle            = lipgloss.NewStyle().Padding(1, 2)
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
	appConfig         AppConfig
	screen            screen
	playlistDir       string
	musicDir          string
	playlists         []Playlist
	songs             []Song
	songsLoaded       bool
	playlistsLoaded   bool
	selectedPlaylist  *Playlist
	selectedSongs     map[string]bool
	selectedOrder     []string // Song paths in selection order
	playlistList      list.Model
	songList          list.Model
	searchInput       textinput.Model
	filteredIndices   []int // Indices into songList.Items() that match search
	existingEntries   map[string]bool
	playlistSongNames []string // Display names of songs already in the playlist
	width             int
	height            int
	err               error
	message           string

	// Device fields
	deviceConnected bool
	devicePath      string // Base mount path, e.g. /Volumes/NO NAME
	confirmEject    bool   // Whether eject confirmation overlay is showing

	// Playlist creation/deletion
	createPlaylistInput textinput.Model
	confirmDelete       bool // Whether delete confirmation overlay is showing

	// Sync fields
	syncSource         string   // Source directory for music sync
	syncInProgress     bool     // Whether sync is running
	syncPendingFiles   []string // Files that would be synced (from dry-run)
	syncConfirmPending bool     // Waiting for y/n confirmation
	syncComplete       bool     // Sync finished successfully
	syncCount          int      // Number of files synced
	syncError          string   // Error message if sync failed
	syncAddToRecent    bool     // Whether to add synced files to Recently Added playlist

	// Podcast fields
	podcastConfig         PodcastConfig
	podcastAudioDir       string // e.g. /Volumes/NO NAME/Audiobooks
	podcastConfigPath     string // e.g. /Volumes/NO NAME/Audiobooks/podcasts.json
	podcastMenuIndex      int    // Selected menu item (0-3)
	podcastSearchInput    textinput.Model
	podcastResults        []iTunesPodcast
	podcastResultIndex    int
	podcastProgress       string              // Current progress message (used by add screen)
	podcastLog            []string            // Log of progress messages
	podcastUpdating       bool                // Whether update/check is in progress
	podcastCheckResult    *PodcastCheckResult // Result from dry-run check
	podcastConfirmPending bool                // Waiting for y/n after check
	podcastUpdateComplete bool                // Download phase finished
	podcastDownloaded     int                 // Number of episodes downloaded
	podcastDeleted        int                 // Number of episodes deleted
	podcastUpdateError    string              // Error during download phase

	// Podcast remove fields
	podcastRemoveNames   []string
	podcastRemoveIndex   int
	podcastConfirmRemove bool

	// Playlist edit mode (right panel of song browser)
	editMode       bool // Whether right panel is in edit mode
	editIndex      int  // Cursor position in right panel
	confirmEditDel bool // Deletion confirmation overlay active
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
type podcastCheckDoneMsg struct {
	err    error
	result PodcastCheckResult
}
type podcastUpdateDoneMsg struct {
	err        error
	downloaded int
	deleted    int
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

	// Playlist creation input
	createTi := textinput.New()
	createTi.Placeholder = "Playlist name..."
	createTi.CharLimit = 50
	createTi.Width = 40

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
		appConfig:           cfg,
		screen:              startScreen,
		deviceConnected:     deviceConnected,
		devicePath:          devicePath,
		playlistDir:         playlistDir,
		musicDir:            musicDir,
		syncSource:          cfg.Sync.Source,
		selectedSongs:       make(map[string]bool),
		existingEntries:     make(map[string]bool),
		searchInput:         ti,
		createPlaylistInput: createTi,
		width:               80,
		height:              24,
		podcastSearchInput:  podcastTi,
		podcastAudioDir:     podcastAudioDir,
		podcastConfigPath:   podcastConfigPath,
		podcastConfig:       make(PodcastConfig),
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

	case podcastCheckDoneMsg:
		m.podcastUpdating = false
		if msg.err != nil {
			m.podcastUpdateError = msg.err.Error()
		} else {
			result := msg.result
			m.podcastCheckResult = &result
			if len(result.Downloads) > 0 || len(result.Deletes) > 0 {
				m.podcastConfirmPending = true
			}
		}
		return m, nil

	case podcastUpdateDoneMsg:
		m.podcastUpdating = false
		m.podcastUpdateComplete = true
		m.podcastDownloaded = msg.downloaded
		m.podcastDeleted = msg.deleted
		if msg.err != nil {
			m.podcastUpdateError = msg.err.Error()
		}
		// Reload config after update
		config, err := LoadPodcastConfig(m.podcastConfigPath)
		if err == nil {
			m.podcastConfig = config
		}
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

	case syncDryRunMsg:
		m.syncInProgress = false
		if msg.err != nil {
			m.syncError = msg.err.Error()
			m.syncPendingFiles = nil
		} else {
			m.syncPendingFiles = msg.files
			m.syncConfirmPending = len(msg.files) > 0
		}
		return m, nil

	case syncDoneMsg:
		m.syncInProgress = false
		if msg.err != nil {
			m.syncError = msg.err.Error()
			m.syncComplete = false
		} else {
			m.syncComplete = true
			m.syncCount = msg.count
		}
		if m.syncAddToRecent && m.syncComplete {
			playlistPath := filepath.Join(m.playlistDir, "Recently Added.m3u8")
			audioExts := map[string]bool{".flac": true, ".mp3": true, ".m4a": true, ".ogg": true, ".wav": true}
			var newEntries []string
			for _, f := range m.syncPendingFiles {
				if !audioExts[strings.ToLower(filepath.Ext(f))] {
					continue
				}
				newEntries = append(newEntries, "../"+m.appConfig.Device.MusicDir+"/"+f)
			}
			if err := UpdateRecentlyAdded(playlistPath, newEntries, 100); err != nil {
				m.syncError = fmt.Sprintf("Synced but failed to update playlist: %v", err)
			}
			m.syncAddToRecent = false
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
		case screenCreatePlaylist:
			return m.updateCreatePlaylist(msg)
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
		case screenPodcastRemove:
			return m.updatePodcastRemove(msg)
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
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "create")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "backup")),
			key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "eject")),
		}
		if syncSource != "" {
			bindings = append(bindings, key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")))
			bindings = append(bindings, key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sync+recent")))
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

	// Handle delete confirmation overlay
	if m.confirmDelete {
		switch {
		case key.Matches(msg, keys.Yes):
			m.confirmDelete = false
			if item, ok := m.playlistList.SelectedItem().(playlistItem); ok {
				if err := os.Remove(item.playlist.Path); err != nil {
					m.message = fmt.Sprintf("Error: %v", err)
				} else {
					m.message = fmt.Sprintf("Deleted %s", item.playlist.Name)
				}
				return m, tea.Batch(
					loadPlaylists(m.playlistDir),
					tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
						return toastDismissMsg{}
					}),
				)
			}
			return m, nil
		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.confirmDelete = false
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
	case msg.String() == "d" && m.playlistList.FilterState() == list.Unfiltered:
		// Only allow delete if a playlist is selected
		if _, ok := m.playlistList.SelectedItem().(playlistItem); ok {
			m.confirmDelete = true
		}
		return m, nil
	case msg.String() == "s" && m.playlistList.FilterState() == list.Unfiltered:
		m.syncAddToRecent = false
		return m.handleSyncKey()
	case msg.String() == "S" && m.playlistList.FilterState() == list.Unfiltered:
		m.syncAddToRecent = true
		return m.handleSyncKey()
	case msg.String() == "b" && m.playlistList.FilterState() == list.Unfiltered:
		return m, m.startBackup()
	case msg.String() == "c" && m.playlistList.FilterState() == list.Unfiltered:
		m.createPlaylistInput.Reset()
		m.createPlaylistInput.Focus()
		m.screen = screenCreatePlaylist
		return m, textinput.Blink
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

			// Load existing entries to filter duplicates and build display names
			entries, _ := LoadPlaylistEntries(item.playlist.Path)
			m.existingEntries = make(map[string]bool)
			for _, e := range entries {
				normalized := NormalizePath(e)
				m.existingEntries[normalized] = true
			}
			m.playlistSongNames = m.buildPlaylistDisplayNames(entries)
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

func (m *Model) buildPlaylistDisplayNames(entries []string) []string {
	// Build a lookup from normalized path -> Song for display names
	songByPath := make(map[string]Song, len(m.songs))
	for _, s := range m.songs {
		relPath := s.RelativePath(m.musicDir)
		songByPath[NormalizePath(relPath)] = s
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		normalized := NormalizePath(e)
		if song, ok := songByPath[normalized]; ok {
			names = append(names, song.ConfirmDisplayName())
		} else {
			// Fallback: use filename without extension
			base := filepath.Base(e)
			names = append(names, strings.TrimSuffix(base, filepath.Ext(base)))
		}
	}
	return names
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
	if m.editMode {
		return m.updateEditMode(msg)
	}

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
		// Intercept 'e' for edit mode (only when not actively searching)
		if msg.String() == "e" && m.searchInput.Value() == "" {
			m.editMode = true
			m.editIndex = 0
			m.confirmEditDel = false
			return m, nil
		}
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
		// Add songs to playlist in selection order
		songsByPath := make(map[string]Song, len(m.songs))
		for _, song := range m.songs {
			songsByPath[song.Path] = song
		}
		var entries []string
		for _, path := range m.selectedOrder {
			if song, ok := songsByPath[path]; ok {
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

func (m Model) updateCreatePlaylist(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = screenPlaylistPicker
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.createPlaylistInput.Value())
		if name == "" {
			return m, nil
		}
		// Create the playlist file
		playlistPath := filepath.Join(m.playlistDir, name+".m3u8")
		// Check if already exists
		if _, err := os.Stat(playlistPath); err == nil {
			m.message = "Playlist already exists"
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return toastDismissMsg{}
			})
		}
		// Create empty file
		f, err := os.Create(playlistPath)
		if err != nil {
			m.message = fmt.Sprintf("Error: %v", err)
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return toastDismissMsg{}
			})
		}
		f.Close()
		// Reload playlists and go back
		m.screen = screenPlaylistPicker
		m.message = fmt.Sprintf("Created %s", name)
		return m, tea.Batch(
			loadPlaylists(m.playlistDir),
			tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return toastDismissMsg{}
			}),
		)
	}
	// Update the text input
	var cmd tea.Cmd
	m.createPlaylistInput, cmd = m.createPlaylistInput.Update(msg)
	return m, cmd
}

func (m Model) viewCreatePlaylist() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Create New Playlist"))
	b.WriteString("\n\n")
	b.WriteString(m.createPlaylistInput.View())
	b.WriteString("\n\n")
	b.WriteString(statusBarStyle.Render("Enter to create, Esc to cancel"))

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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
		m.syncError = fmt.Sprintf("Source not found: %s", m.syncSource)
		m.syncPendingFiles = nil
		m.syncConfirmPending = false
		m.syncComplete = false
		m.syncInProgress = false
		m.screen = screenSync
		return m, nil
	}

	// Source exists — start dry-run to see what would be synced
	m.syncInProgress = true
	m.syncError = ""
	m.syncPendingFiles = nil
	m.syncConfirmPending = false
	m.syncComplete = false
	m.screen = screenSync
	return m, runSyncDryRun(m.syncSource, m.musicDir)
}

func (m Model) updateSync(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	// If sync is in progress, ignore all keys
	if m.syncInProgress {
		return m, nil
	}

	// If waiting for confirmation after dry-run
	if m.syncConfirmPending {
		switch {
		case key.Matches(msg, keys.Yes):
			// Run actual sync
			m.syncInProgress = true
			m.syncConfirmPending = false
			return m, runSync(m.syncSource, m.musicDir, len(m.syncPendingFiles))
		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.screen = screenPlaylistPicker
			m.syncPendingFiles = nil
			m.syncConfirmPending = false
			return m, nil
		}
		return m, nil
	}

	// Sync complete or error — any key exits
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.screen = screenPlaylistPicker
		m.syncPendingFiles = nil
		m.syncComplete = false
		m.syncError = ""
		m.syncAddToRecent = false
		return m, nil
	}
	return m, nil
}

// Podcast menu items
var podcastMenuItems = []string{
	"Update all podcasts",
	"Add new podcast",
	"Remove podcast",
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
			m.podcastUpdating = true
			m.podcastCheckResult = nil
			m.podcastConfirmPending = false
			m.podcastUpdateComplete = false
			m.podcastDownloaded = 0
			m.podcastDeleted = 0
			m.podcastUpdateError = ""
			return m, m.startPodcastCheck()
		case 1: // Add new podcast
			m.screen = screenPodcastSearch
			m.podcastSearchInput.SetValue("")
			m.podcastSearchInput.Focus()
			m.podcastResults = nil
			return m, nil
		case 2: // Remove podcast
			var names []string
			for name := range m.podcastConfig {
				names = append(names, name)
			}
			sort.Strings(names)
			m.podcastRemoveNames = names
			m.podcastRemoveIndex = 0
			m.podcastConfirmRemove = false
			m.screen = screenPodcastRemove
			return m, nil
		case 3: // Browse & add songs
			// Load existing entries to filter duplicates
			entries, _ := LoadPlaylistEntries(m.selectedPlaylist.Path)
			m.existingEntries = make(map[string]bool)
			for _, e := range entries {
				normalized := NormalizePath(e)
				m.existingEntries[normalized] = true
			}
			m.playlistSongNames = m.buildPlaylistDisplayNames(entries)
			m.setupSongList()
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.screen = screenSongBrowser
			return m, nil
		case 4: // Back
			m.screen = screenPlaylistPicker
			return m, nil
		}
	case tea.KeyEsc:
		m.screen = screenPlaylistPicker
		return m, nil
	}
	return m, nil
}

func (m Model) startPodcastCheck() tea.Cmd {
	config := m.podcastConfig
	audioDir := m.podcastAudioDir
	episodesToKeep := m.appConfig.Podcast.EpisodesToKeep

	return func() tea.Msg {
		if len(config) == 0 {
			return podcastCheckDoneMsg{result: PodcastCheckResult{}}
		}

		result := CheckPodcastUpdates(config, audioDir, episodesToKeep)
		return podcastCheckDoneMsg{result: result}
	}
}

func (m Model) startPodcastDownload() tea.Cmd {
	checkResult := m.podcastCheckResult
	config := m.podcastConfig
	audioDir := m.podcastAudioDir
	configPath := m.podcastConfigPath
	playlistPath := m.selectedPlaylist.Path
	episodesToKeep := m.appConfig.Podcast.EpisodesToKeep

	return func() tea.Msg {
		downloaded, deleted, err := ExecutePodcastUpdates(*checkResult, config, audioDir, configPath, playlistPath, episodesToKeep)
		return podcastUpdateDoneMsg{err: err, downloaded: downloaded, deleted: deleted}
	}
}

func (m Model) updatePodcastUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	// If update/check is in progress, ignore all keys
	if m.podcastUpdating {
		return m, nil
	}

	// If waiting for confirmation after check
	if m.podcastConfirmPending {
		switch {
		case key.Matches(msg, keys.Yes):
			// Start actual download
			m.podcastUpdating = true
			m.podcastConfirmPending = false
			return m, m.startPodcastDownload()
		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.screen = screenPodcastMenu
			m.podcastCheckResult = nil
			m.podcastConfirmPending = false
			return m, nil
		}
		return m, nil
	}

	// Update complete or nothing new — any key exits
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.screen = screenPodcastMenu
		m.podcastCheckResult = nil
		m.podcastUpdateComplete = false
		m.podcastUpdateError = ""
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
	case screenCreatePlaylist:
		return m.viewCreatePlaylist()
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
	case screenPodcastRemove:
		return m.viewPodcastRemove()
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
		toast := dangerConfirmStyle.Render("Eject player? (y/n)")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, toast)
	}

	if m.confirmDelete {
		if item, ok := m.playlistList.SelectedItem().(playlistItem); ok {
			toast := dangerConfirmStyle.Render(fmt.Sprintf("Delete %s? (y/n)", item.playlist.Name))
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, toast)
		}
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
	var statusText string
	if m.editMode {
		statusText = fmt.Sprintf("%d selected | editing right panel | esc: stop editing", selectedCount)
	} else {
		statusText = fmt.Sprintf("%d selected | tab: select | e: edit | enter: confirm | esc: back/clear", selectedCount)
	}
	status := statusBarStyle.Render(statusText)
	left.WriteString(status)
	left.WriteString("\n\n")

	// Song list
	left.WriteString(m.songList.View())

	leftPanel := lipgloss.NewStyle().Width(leftWidth).Render(left.String())

	// === Right panel: playlist contents ===
	var right strings.Builder

	if m.editMode {
		right.WriteString(selectedTitleStyle.Render(fmt.Sprintf("Edit: %s (%d)", m.selectedPlaylist.Name, len(m.playlistSongNames))))
		right.WriteString("\n\n")

		if len(m.playlistSongNames) == 0 {
			right.WriteString(selectedDimStyle.Render("Playlist is empty"))
			right.WriteString("\n\n")
			right.WriteString(statusBarStyle.Render("esc: back"))
		} else {
			maxVisible := m.height - 7
			if maxVisible < 3 {
				maxVisible = 3
			}
			start := 0
			if m.editIndex >= maxVisible {
				start = m.editIndex - maxVisible + 1
			}
			end := start + maxVisible
			if end > len(m.playlistSongNames) {
				end = len(m.playlistSongNames)
			}

			for i := start; i < end; i++ {
				if i == m.editIndex {
					right.WriteString(podcastSelectedStyle.Render("> " + m.playlistSongNames[i]))
				} else {
					right.WriteString(selectedSongStyle.Render("  " + m.playlistSongNames[i]))
				}
				right.WriteString("\n")
			}

			right.WriteString("\n")
			if m.message != "" {
				right.WriteString(toastStyle.Render(m.message))
			} else if m.confirmEditDel {
				name := m.playlistSongNames[m.editIndex]
				right.WriteString(dangerConfirmStyle.Render(fmt.Sprintf("Remove \"%s\"? (y/n)", name)))
			} else {
				right.WriteString(statusBarStyle.Render("d: remove | esc: back"))
			}
		}
	} else {
		totalCount := len(m.playlistSongNames) + selectedCount
		right.WriteString(selectedTitleStyle.Render(fmt.Sprintf("%s (%d)", m.selectedPlaylist.Name, totalCount)))
		right.WriteString("\n\n")

		// Build selected song display names for the + lines
		songMap := make(map[string]Song, len(m.songs))
		for _, s := range m.songs {
			songMap[s.Path] = s
		}
		var addedNames []string
		for _, path := range m.selectedOrder {
			if song, ok := songMap[path]; ok {
				addedNames = append(addedNames, song.ConfirmDisplayName())
			}
		}

		maxVisible := m.height - 5
		if maxVisible < 1 {
			maxVisible = 1
		}
		lineCount := 0

		// Show existing playlist songs first
		for _, name := range m.playlistSongNames {
			if lineCount >= maxVisible {
				remaining := len(m.playlistSongNames) - lineCount + len(addedNames)
				right.WriteString(selectedDimStyle.Render(fmt.Sprintf("... and %d more", remaining)))
				break
			}
			right.WriteString(selectedSongStyle.Render("  " + name))
			right.WriteString("\n")
			lineCount++
		}

		// Show newly selected songs at the end with green + prefix
		for _, name := range addedNames {
			if lineCount >= maxVisible {
				remaining := len(addedNames) - (lineCount - len(m.playlistSongNames))
				right.WriteString(selectedDimStyle.Render(fmt.Sprintf("... and %d more", remaining)))
				lineCount++
				break
			}
			right.WriteString(addedSongStyle.Render("+ " + name))
			right.WriteString("\n")
			lineCount++
		}

		if totalCount == 0 {
			right.WriteString(selectedDimStyle.Render("Empty playlist"))
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

	// List selected songs in selection order
	songsByPath := make(map[string]Song, len(m.songs))
	for _, song := range m.songs {
		songsByPath[song.Path] = song
	}
	for _, path := range m.selectedOrder {
		if song, ok := songsByPath[path]; ok {
			b.WriteString(songListStyle.Render("• " + song.ConfirmDisplayName()))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("Press y to confirm, n to cancel"))

	return b.String()
}

func (m Model) viewSync() string {
	var b strings.Builder

	b.WriteString(syncTitleStyle.Render("Syncing Music"))
	b.WriteString("\n\n")

	if m.syncInProgress {
		if m.syncConfirmPending || len(m.syncPendingFiles) == 0 {
			b.WriteString(syncOutputStyle.Render("Checking for new songs..."))
		} else {
			b.WriteString(syncOutputStyle.Render("Syncing, please wait..."))
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("This may take a while..."))
	} else if m.syncError != "" {
		// Error occurred
		b.WriteString(syncOutputStyle.Render("Error: " + m.syncError))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	} else if m.syncComplete {
		// Sync completed successfully
		if m.syncCount == 1 {
			b.WriteString(syncOutputStyle.Render("1 song added to player"))
		} else {
			b.WriteString(syncOutputStyle.Render(fmt.Sprintf("%d songs added to player", m.syncCount)))
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	} else if m.syncConfirmPending {
		// Dry-run complete, show files to be synced
		count := len(m.syncPendingFiles)
		if count == 1 {
			b.WriteString(syncOutputStyle.Render("1 song will be added:"))
		} else {
			b.WriteString(syncOutputStyle.Render(fmt.Sprintf("%d songs will be added:", count)))
		}
		b.WriteString("\n\n")

		// Show file list, truncated if too many
		maxLines := m.height - 10
		if maxLines < 5 {
			maxLines = 5
		}
		if maxLines > 20 {
			maxLines = 20
		}

		for i, file := range m.syncPendingFiles {
			if i >= maxLines {
				remaining := count - maxLines
				b.WriteString(syncOutputStyle.Render(fmt.Sprintf("  ...and %d more", remaining)))
				b.WriteString("\n")
				break
			}
			b.WriteString(syncOutputStyle.Render("  " + file))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(statusBarStyle.Render("Press y to sync, n to cancel"))
	} else {
		// No files to sync (dry-run returned empty)
		b.WriteString(syncOutputStyle.Render("Music is already up to date"))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	}

	return wrapStyle.Render(b.String())
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

	return wrapStyle.Render(b.String())
}

func (m Model) viewPodcastUpdate() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Updating Podcasts"))
	b.WriteString("\n\n")

	if m.podcastUpdating {
		if m.podcastConfirmPending || m.podcastCheckResult == nil {
			// Phase 1: checking RSS feeds
			b.WriteString(podcastProgressStyle.Render("Checking for new episodes..."))
		} else {
			// Phase 2: downloading
			b.WriteString(podcastProgressStyle.Render("Downloading, please wait..."))
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("This may take a while..."))
	} else if m.podcastUpdateError != "" {
		b.WriteString(podcastProgressStyle.Render("Error: " + m.podcastUpdateError))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	} else if m.podcastUpdateComplete {
		// Download finished
		if m.podcastDownloaded == 0 && m.podcastDeleted == 0 {
			b.WriteString(podcastProgressStyle.Render("Nothing to update"))
		} else {
			if m.podcastDownloaded == 1 {
				b.WriteString(podcastProgressStyle.Render("1 episode downloaded"))
			} else if m.podcastDownloaded > 1 {
				b.WriteString(podcastProgressStyle.Render(fmt.Sprintf("%d episodes downloaded", m.podcastDownloaded)))
			}
			if m.podcastDeleted > 0 {
				if m.podcastDownloaded > 0 {
					b.WriteString("\n")
				}
				if m.podcastDeleted == 1 {
					b.WriteString(podcastProgressStyle.Render("1 old episode removed"))
				} else {
					b.WriteString(podcastProgressStyle.Render(fmt.Sprintf("%d old episodes removed", m.podcastDeleted)))
				}
			}
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	} else if m.podcastConfirmPending && m.podcastCheckResult != nil {
		result := m.podcastCheckResult

		// Show new episodes to download
		if len(result.Downloads) > 0 {
			if len(result.Downloads) == 1 {
				b.WriteString(podcastProgressStyle.Render("1 new episode:"))
			} else {
				b.WriteString(podcastProgressStyle.Render(fmt.Sprintf("%d new episodes:", len(result.Downloads))))
			}
			b.WriteString("\n\n")

			// Group by podcast name
			grouped := make(map[string][]PendingEpisode)
			var order []string
			for _, ep := range result.Downloads {
				if _, exists := grouped[ep.PodcastName]; !exists {
					order = append(order, ep.PodcastName)
				}
				grouped[ep.PodcastName] = append(grouped[ep.PodcastName], ep)
			}

			maxLines := m.height - 14
			if maxLines < 5 {
				maxLines = 5
			}
			lines := 0

			for _, name := range order {
				if lines >= maxLines {
					remaining := len(result.Downloads) - lines
					b.WriteString(podcastProgressStyle.Render(fmt.Sprintf("  ...and %d more", remaining)))
					b.WriteString("\n")
					break
				}
				b.WriteString(podcastSelectedStyle.Render("  " + name))
				b.WriteString("\n")
				lines++
				for _, ep := range grouped[name] {
					if lines >= maxLines {
						break
					}
					b.WriteString(podcastProgressStyle.Render("    " + ep.Title))
					b.WriteString("\n")
					lines++
				}
			}
		}

		// Show deletions
		if len(result.Deletes) > 0 {
			if len(result.Downloads) > 0 {
				b.WriteString("\n")
			}
			if len(result.Deletes) == 1 {
				b.WriteString(podcastProgressStyle.Render("1 old episode to remove"))
			} else {
				b.WriteString(podcastProgressStyle.Render(fmt.Sprintf("%d old episodes to remove", len(result.Deletes))))
			}
			b.WriteString("\n")
		}

		// Show errors
		if len(result.Errors) > 0 {
			b.WriteString("\n")
			for _, e := range result.Errors {
				b.WriteString(podcastProgressStyle.Render("  Error: " + e))
				b.WriteString("\n")
			}
		}

		b.WriteString("\n")
		b.WriteString(statusBarStyle.Render("Press y to download, n to cancel"))
	} else if m.podcastCheckResult != nil {
		// Check done, nothing new
		if len(m.podcastCheckResult.Errors) > 0 {
			b.WriteString(podcastProgressStyle.Render("Podcasts are up to date"))
			b.WriteString("\n\n")
			for _, e := range m.podcastCheckResult.Errors {
				b.WriteString(podcastProgressStyle.Render("  Error: " + e))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(podcastProgressStyle.Render("Podcasts are up to date"))
		}
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	} else {
		// No podcasts subscribed
		b.WriteString(podcastProgressStyle.Render("No podcasts subscribed"))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("Press enter or esc to continue"))
	}

	return wrapStyle.Render(b.String())
}

func (m Model) viewPodcastSearch() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Search Podcasts"))
	b.WriteString("\n")
	b.WriteString("Search: " + m.podcastSearchInput.View())
	b.WriteString("\n\n")
	b.WriteString(statusBarStyle.Render("enter: search | esc: back"))

	return wrapStyle.Render(b.String())
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

	return wrapStyle.Render(b.String())
}

func (m Model) updatePodcastRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.podcastConfirmRemove {
		switch {
		case key.Matches(msg, keys.Yes):
			name := m.podcastRemoveNames[m.podcastRemoveIndex]
			RemovePodcast(name, m.podcastConfig, m.podcastAudioDir)
			SavePodcastConfig(m.podcastConfigPath, m.podcastConfig)
			if m.selectedPlaylist != nil {
				RebuildPodcastPlaylist(m.podcastConfig, m.selectedPlaylist.Path, m.podcastAudioDir)
			}
			m.podcastConfirmRemove = false
			m.message = fmt.Sprintf("Removed \"%s\"", name)
			m.screen = screenPodcastMenu
			m.podcastMenuIndex = 0
			return m, nil
		case key.Matches(msg, keys.No), msg.Type == tea.KeyEsc:
			m.podcastConfirmRemove = false
			return m, nil
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyUp:
		if m.podcastRemoveIndex > 0 {
			m.podcastRemoveIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.podcastRemoveIndex < len(m.podcastRemoveNames)-1 {
			m.podcastRemoveIndex++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.podcastRemoveNames) > 0 {
			m.podcastConfirmRemove = true
		}
		return m, nil
	case tea.KeyEsc:
		m.screen = screenPodcastMenu
		return m, nil
	}
	return m, nil
}

func (m Model) viewPodcastRemove() string {
	var b strings.Builder

	b.WriteString(podcastTitleStyle.Render("Remove Podcast"))
	b.WriteString("\n")

	if len(m.podcastRemoveNames) == 0 {
		b.WriteString(podcastItemStyle.Render("No podcasts subscribed"))
		b.WriteString("\n\n")
		b.WriteString(statusBarStyle.Render("esc: back"))
		return wrapStyle.Render(b.String())
	}

	for i, name := range m.podcastRemoveNames {
		if i == m.podcastRemoveIndex {
			b.WriteString(podcastSelectedStyle.Render("> " + name))
		} else {
			b.WriteString(podcastItemStyle.Render("  " + name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.podcastConfirmRemove {
		name := m.podcastRemoveNames[m.podcastRemoveIndex]
		b.WriteString(dangerConfirmStyle.Render(fmt.Sprintf("Remove \"%s\"? This deletes all episodes. (y/n)", name)))
	} else {
		b.WriteString(statusBarStyle.Render("enter: select | esc: back"))
	}

	return wrapStyle.Render(b.String())
}

func (m Model) updateEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmEditDel {
		switch {
		case key.Matches(msg, keys.Yes):
			name := m.playlistSongNames[m.editIndex]
			// Remove from playlistSongNames and rebuild existingEntries
			entries, _ := LoadPlaylistEntries(m.selectedPlaylist.Path)
			if m.editIndex < len(entries) {
				entries = append(entries[:m.editIndex], entries[m.editIndex+1:]...)
			}
			if err := WritePlaylist(m.selectedPlaylist.Path, entries); err != nil {
				m.err = err
				return m, tea.Quit
			}
			// Reload state from disk
			m.existingEntries = make(map[string]bool)
			for _, e := range entries {
				m.existingEntries[NormalizePath(e)] = true
			}
			m.playlistSongNames = m.buildPlaylistDisplayNames(entries)
			m.filterSongs()
			m.confirmEditDel = false
			m.message = fmt.Sprintf("Removed \"%s\"", name)
			if len(m.playlistSongNames) == 0 {
				m.editMode = false
			} else if m.editIndex >= len(m.playlistSongNames) {
				m.editIndex = len(m.playlistSongNames) - 1
			}
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return toastDismissMsg{}
			})
		case key.Matches(msg, keys.No), msg.Type == tea.KeyEsc:
			m.confirmEditDel = false
			return m, nil
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyUp:
		if m.editIndex > 0 {
			m.editIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.editIndex < len(m.playlistSongNames)-1 {
			m.editIndex++
		}
		return m, nil
	case tea.KeyRunes:
		if msg.String() == "d" && len(m.playlistSongNames) > 0 {
			m.confirmEditDel = true
		}
		return m, nil
	case tea.KeyEsc:
		m.editMode = false
		m.confirmEditDel = false
		return m, nil
	}
	return m, nil
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

	return wrapStyle.Render(b.String())
}

func main() {
	// Subcommands (handled before flag parsing)
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "config" {
		fmt.Print(defaultConfigTOML)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		cfg := LoadConfig()
		var devicePath string
		if len(os.Args) > 2 {
			devicePath = os.Args[2]
		} else if cfg.Device.Path != "" {
			devicePath = cfg.Device.Path
		} else {
			devicePath = FindDevicePath(cfg.Device.SearchPaths, cfg.Device.MusicDir, cfg.Device.PlaylistDir)
		}
		if devicePath == "" {
			fmt.Fprintln(os.Stderr, "No device found. Usage: rockbox-playlist doctor [device-path]")
			os.Exit(1)
		}
		runDoctor(
			filepath.Join(devicePath, cfg.Device.PlaylistDir),
			filepath.Join(devicePath, cfg.Device.MusicDir),
		)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "led" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: rockbox-playlist led <on|off|blink|heartbeat>")
			os.Exit(1)
		}
		var err error
		switch os.Args[2] {
		case "on":
			err = SetLEDOn()
		case "off":
			err = SetLEDOff()
		case "blink":
			err = SetLEDBlink(300 * time.Millisecond)
		case "heartbeat":
			err = SetLEDHeartbeat()
		default:
			fmt.Fprintf(os.Stderr, "Unknown LED mode: %s\n", os.Args[2])
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "led-sos" {
		if err := BlinkSOSForever(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "autosync-skip" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: rockbox-playlist autosync-skip <on|off|status>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "on":
			if err := enableAutosyncSkip(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println(autosyncSkipStatus())
		case "off":
			if err := disableAutosyncSkip(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println(autosyncSkipStatus())
		case "status":
			fmt.Println(autosyncSkipStatus())
		default:
			fmt.Fprintf(os.Stderr, "Unknown autosync-skip mode: %s\n", os.Args[2])
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "autosync" {
		cfg := LoadConfig()
		fs := flag.NewFlagSet("autosync", flag.ExitOnError)
		devicePath := fs.String("device-path", cfg.Device.Path, "Path to Rockbox device")
		syncSource := fs.String("sync-source", cfg.Sync.Source, "Source directory for music sync")
		lockPath := fs.String("lock-file", filepath.Join(os.TempDir(), "rockbox-playlist-autosync.lock"), "Lock file path")
		statusPath := fs.String("status-file", DefaultAutosyncStatusPath(), "Autosync status file path")
		fs.Parse(os.Args[2:])

		unlock, err := AcquireLock(*lockPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer unlock()

		summary, err := RunAutosync(cfg, *devicePath, *syncSource, *statusPath, func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Printf(
			"Autosync complete: synced=%d recently_added=%d podcasts_downloaded=%d podcasts_deleted=%d\n",
			summary.SyncCount,
			summary.RecentlyAddedCount,
			summary.PodcastDownloaded,
			summary.PodcastDeleted,
		)
		return
	}

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
