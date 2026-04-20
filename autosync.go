package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const recentlyAddedLimit = 100

type autosyncSummary struct {
	SyncCount          int
	RecentlyAddedCount int
	PodcastDownloaded  int
	PodcastDeleted     int
}

type autosyncStatus struct {
	State              string    `json:"state"`
	Phase              string    `json:"phase"`
	Message            string    `json:"message,omitempty"`
	LastError          string    `json:"last_error,omitempty"`
	StartedAt          time.Time `json:"started_at,omitempty"`
	FinishedAt         time.Time `json:"finished_at,omitempty"`
	SyncCount          int       `json:"sync_count"`
	RecentlyAddedCount int       `json:"recently_added_count"`
	PodcastDownloaded  int       `json:"podcasts_downloaded"`
	PodcastDeleted     int       `json:"podcasts_deleted"`
}

func autosyncSkipReason() string {
	switch {
	case sessionActive():
		return "interactive session active"
	case autosyncSkipEnabled():
		return "manual autosync skip enabled"
	default:
		return ""
	}
}

func autosyncMountHoldReason() string {
	if sessionActive() {
		return "interactive session active; leaving device mounted"
	}
	return ""
}

func DefaultAutosyncStatusPath() string {
	return filepath.Join(runtimeStateDir(), "autosync-status.json")
}

func writeAutosyncStatus(path string, status autosyncStatus) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func logPhase(logf func(string, ...any), phase string, format string, args ...any) {
	if logf == nil {
		return
	}
	logf("[%s] "+format, append([]any{phase}, args...)...)
}

func BuildRecentlyAddedEntries(files []string, musicDir string) []string {
	audioExts := map[string]bool{
		".flac": true,
		".m4a":  true,
		".mp3":  true,
		".ogg":  true,
		".wav":  true,
	}

	var entries []string
	for _, f := range files {
		if !audioExts[strings.ToLower(filepath.Ext(f))] {
			continue
		}
		entries = append(entries, "../"+musicDir+"/"+f)
	}

	return entries
}

func UpdateRecentlyAddedFromSync(playlistPath string, files []string, musicDir string) (int, error) {
	entries := BuildRecentlyAddedEntries(files, musicDir)
	if len(entries) == 0 {
		return 0, nil
	}
	if err := UpdateRecentlyAdded(playlistPath, entries, recentlyAddedLimit); err != nil {
		return 0, err
	}
	return len(entries), nil
}

func FindPodcastPlaylistPath(playlistDir string) (string, error) {
	playlists, err := LoadPlaylists(playlistDir)
	if err != nil {
		return "", err
	}

	for _, playlist := range playlists {
		if IsPodcastPlaylist(playlist.Name) {
			return playlist.Path, nil
		}
	}

	return "", fmt.Errorf("no podcast playlist found in %s", playlistDir)
}

func RefreshPodcasts(config AppConfig, playlistDir, audioDir string, logf func(string, ...any)) (downloaded int, deleted int, err error) {
	configPath := filepath.Join(audioDir, "podcasts.json")
	podcastConfig, err := LoadPodcastConfig(configPath)
	if err != nil {
		return 0, 0, fmt.Errorf("loading podcast config: %w", err)
	}
	if len(podcastConfig) == 0 {
		logPhase(logf, "podcast", "no subscriptions configured")
		return 0, 0, nil
	}

	playlistPath, err := FindPodcastPlaylistPath(playlistDir)
	if err != nil {
		return 0, 0, fmt.Errorf("finding podcast playlist: %w", err)
	}

	checkResult := CheckPodcastUpdates(podcastConfig, audioDir, config.Podcast.EpisodesToKeep)
	logPhase(logf, "podcast", "checked %d subscription(s): %d download(s), %d delete(s)", len(podcastConfig), len(checkResult.Downloads), len(checkResult.Deletes))
	if len(checkResult.Errors) > 0 {
		for _, checkErr := range checkResult.Errors {
			logPhase(logf, "podcast", "check warning: %s", checkErr)
		}
	}

	downloaded, deleted, err = ExecutePodcastUpdates(
		checkResult,
		podcastConfig,
		audioDir,
		configPath,
		playlistPath,
		config.Podcast.EpisodesToKeep,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("executing podcast updates: %w", err)
	}
	logPhase(logf, "podcast", "completed: %d download(s), %d delete(s)", downloaded, deleted)
	return downloaded, deleted, nil
}

func AcquireLock(path string) (func(), error) {
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, fmt.Errorf("autosync already running")
		}
		return nil, err
	}

	return func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}, nil
}

func EjectDevicePath(path string) error {
	msg := EjectDevice(path)()
	ejectMsg, ok := msg.(deviceEjectMsg)
	if !ok {
		return fmt.Errorf("unexpected eject result: %T", msg)
	}
	return ejectMsg.err
}

func RunAutosync(cfg AppConfig, devicePath, syncSource, statusPath string, logf func(string, ...any)) (autosyncSummary, error) {
	var summary autosyncSummary
	status := autosyncStatus{
		State:     "running",
		Phase:     "starting",
		StartedAt: time.Now(),
	}

	if logf == nil {
		logf = func(string, ...any) {}
	}
	if statusPath == "" {
		statusPath = DefaultAutosyncStatusPath()
	}
	updateStatus := func(phase, state, message string) {
		status.Phase = phase
		if state != "" {
			status.State = state
		}
		status.Message = message
		status.SyncCount = summary.SyncCount
		status.RecentlyAddedCount = summary.RecentlyAddedCount
		status.PodcastDownloaded = summary.PodcastDownloaded
		status.PodcastDeleted = summary.PodcastDeleted
		if err := writeAutosyncStatus(statusPath, status); err != nil {
			logPhase(logf, "status", "write failed: %v", err)
		}
	}
	fail := func(phase string, err error) (autosyncSummary, error) {
		status.State = "failed"
		status.Phase = phase
		status.Message = "autosync failed"
		status.LastError = err.Error()
		status.FinishedAt = time.Now()
		status.SyncCount = summary.SyncCount
		status.RecentlyAddedCount = summary.RecentlyAddedCount
		status.PodcastDownloaded = summary.PodcastDownloaded
		status.PodcastDeleted = summary.PodcastDeleted
		if writeErr := writeAutosyncStatus(statusPath, status); writeErr != nil {
			logPhase(logf, "status", "write failed: %v", writeErr)
		}
		logPhase(logf, phase, "failed: %v", err)
		return summary, err
	}
	updateStatus("starting", "running", "autosync starting")

	if devicePath == "" {
		devicePath = cfg.Device.Path
	}
	if syncSource == "" {
		syncSource = cfg.Sync.Source
	}

	if devicePath == "" {
		return fail("starting", fmt.Errorf("no device path configured"))
	}
	if syncSource == "" {
		return fail("starting", fmt.Errorf("no sync source configured"))
	}
	logPhase(logf, "led", "setting solid on")
	if err := SetLEDOn(); err != nil {
		return fail("led", fmt.Errorf("setting LED on: %w", err))
	}
	if !CheckDevice(devicePath, cfg.Device.MusicDir, cfg.Device.PlaylistDir, cfg.Device.AudiobooksDir) {
		return fail("starting", fmt.Errorf("device not available at %s", devicePath))
	}
	if info, err := os.Stat(syncSource); err != nil || !info.IsDir() {
		return fail("starting", fmt.Errorf("sync source not available at %s", syncSource))
	}

	playlistDir := filepath.Join(devicePath, cfg.Device.PlaylistDir)
	musicDir := filepath.Join(devicePath, cfg.Device.MusicDir)
	audioDir := filepath.Join(devicePath, cfg.Device.AudiobooksDir)
	for _, dir := range []string{playlistDir, musicDir, audioDir} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fail("starting", fmt.Errorf("required device directory missing: %s", dir))
		}
	}
	if reason := autosyncSkipReason(); reason != "" {
		logPhase(logf, "led", "setting off")
		if err := SetLEDOff(); err != nil {
			return fail("led", fmt.Errorf("setting LED off: %w", err))
		}
		updateStatus("skipped", "skipped", reason)
		logPhase(logf, "skip", reason)
		return summary, nil
	}

	updateStatus("preflight", "running", "device mounted, preparing sync")
	logPhase(logf, "led", "setting blink")
	if err := SetLEDBlink(300 * time.Millisecond); err != nil {
		return fail("led", fmt.Errorf("setting LED blink: %w", err))
	}

	updateStatus("sync_preview", "running", "previewing music sync")
	logPhase(logf, "sync", "previewing from %s to %s", syncSource, musicDir)
	files, err := SyncPreview(syncSource, musicDir)
	if err != nil {
		return fail("sync_preview", err)
	}
	logPhase(logf, "sync", "preview found %d file(s)", len(files))

	if len(files) > 0 {
		updateStatus("sync", "running", fmt.Sprintf("syncing %d file(s)", len(files)))
		logPhase(logf, "sync", "syncing %d file(s)", len(files))
		if err := SyncFiles(syncSource, musicDir); err != nil {
			return fail("sync", err)
		}
		summary.SyncCount = len(files)

		recentlyAddedPath := filepath.Join(playlistDir, "Recently Added.m3u8")
		updateStatus("recently_added", "running", "updating Recently Added")
		logPhase(logf, "recent", "updating %s", recentlyAddedPath)
		recentlyAddedCount, err := UpdateRecentlyAddedFromSync(recentlyAddedPath, files, cfg.Device.MusicDir)
		if err != nil {
			return fail("recently_added", fmt.Errorf("synced but failed to update Recently Added: %w", err))
		}
		summary.RecentlyAddedCount = recentlyAddedCount
		logPhase(logf, "recent", "added %d entrie(s)", recentlyAddedCount)
	} else {
		logPhase(logf, "sync", "no music changes detected")
	}

	updateStatus("podcasts", "running", "refreshing podcasts")
	logPhase(logf, "podcast", "refreshing subscriptions")
	downloaded, deleted, err := RefreshPodcasts(cfg, playlistDir, audioDir, logf)
	if err != nil {
		return fail("podcasts", err)
	}
	summary.PodcastDownloaded = downloaded
	summary.PodcastDeleted = deleted

	if reason := autosyncMountHoldReason(); reason != "" {
		logPhase(logf, "led", "setting off")
		if err := SetLEDOff(); err != nil {
			return fail("led", fmt.Errorf("setting LED off: %w", err))
		}
		updateStatus("done", "done", reason)
		logPhase(logf, "skip", reason)
		logPhase(logf, "done", "complete without unmount: synced=%d recently_added=%d podcasts_downloaded=%d podcasts_deleted=%d",
			summary.SyncCount,
			summary.RecentlyAddedCount,
			summary.PodcastDownloaded,
			summary.PodcastDeleted,
		)
		return summary, nil
	}

	updateStatus("unmount", "running", "unmounting device")
	logPhase(logf, "unmount", "unmounting %s", devicePath)
	if err := EjectDevicePath(devicePath); err != nil {
		return fail("unmount", err)
	}
	logPhase(logf, "led", "setting off")
	if err := SetLEDOff(); err != nil {
		return fail("led", fmt.Errorf("setting LED off: %w", err))
	}
	status.State = "done"
	status.Phase = "done"
	status.Message = "autosync complete"
	status.LastError = ""
	status.FinishedAt = time.Now()
	updateStatus("done", "done", "autosync complete")
	logPhase(logf, "done", "complete: synced=%d recently_added=%d podcasts_downloaded=%d podcasts_deleted=%d",
		summary.SyncCount,
		summary.RecentlyAddedCount,
		summary.PodcastDownloaded,
		summary.PodcastDeleted,
	)

	return summary, nil
}
