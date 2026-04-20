package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	wishbt "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

// ServerConfig holds configuration for the SSH server.
type ServerConfig struct {
	Host       string
	Port       string
	HostKeyDir string
	DevicePath string    // Base path to the Rockbox device (e.g. /Volumes/NO NAME)
	SyncSource string    // Source directory for music sync
	AppCfg     AppConfig // Full app config to pass to sessions
}

// sessionGuard enforces single-session access.
type sessionGuard struct {
	mu     sync.Mutex
	active bool
}

func (g *sessionGuard) acquire() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active {
		return false
	}
	if err := markSessionActive(); err != nil {
		log.Error("Could not mark session active", "error", err)
		return false
	}
	g.active = true
	return true
}

func (g *sessionGuard) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active = false
	if err := clearSessionActive(); err != nil {
		log.Error("Could not clear session marker", "error", err)
	}
}

var guard = &sessionGuard{}

// singleSessionMiddleware rejects additional SSH connections when one is active.
func singleSessionMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if !guard.acquire() {
				fmt.Fprintln(s, "Another session is already active. Try again later.")
				_ = s.Exit(1)
				return
			}
			defer guard.release()
			next(s)
		}
	}
}

// makeTeaHandler creates a new Bubbletea model for each SSH session.
func makeTeaHandler(cfg AppConfig, devicePath string) func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		pty, _, _ := s.Pty()
		m := initialModelServe(cfg, devicePath, pty.Window.Width, pty.Window.Height)
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}
}

// StartServer starts the Wish SSH server.
func StartServer(cfg ServerConfig) {
	// Ensure host key directory exists
	if err := os.MkdirAll(cfg.HostKeyDir, 0700); err != nil {
		log.Error("Could not create host key directory", "path", cfg.HostKeyDir, "error", err)
		os.Exit(1)
	}

	hostKeyPath := cfg.HostKeyDir + "/id_ed25519"

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(cfg.Host, cfg.Port)),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithMiddleware(
			wishbt.Middleware(makeTeaHandler(cfg.AppCfg, cfg.DevicePath)),
			activeterm.Middleware(),
			singleSessionMiddleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not create server", "error", err)
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Starting SSH server", "host", cfg.Host, "port", cfg.Port)
	if cfg.DevicePath != "" {
		log.Info("Device path", "path", cfg.DevicePath)
	} else {
		log.Info("Device path not set, will auto-detect")
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Server error", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server gracefully", "error", err)
	}
}
