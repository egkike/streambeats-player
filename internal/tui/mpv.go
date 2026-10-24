package tui

// mpv process wiring: the TUI needs a launchable headless mpv that exposes its
// JSON IPC socket. Spawning is best effort — when mpv is missing the TUI still
// works for browsing and reports a friendly status line instead of crashing.
// No unit test starts a process; only the pure argument/path helpers are tested.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"streambeats/internal/player"
)

const (
	// mpvStartupTimeout bounds how long we wait for the IPC socket.
	mpvStartupTimeout = 5 * time.Second
	// mpvDialTimeout bounds a single IPC connection attempt.
	mpvDialTimeout = 2 * time.Second
	socketName     = "streambeats-mpv.sock"
)

// DefaultSocketPath returns a per-user runtime unix socket path for mpv IPC.
func DefaultSocketPath() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, socketName), nil
	}
	tmp := os.TempDir()
	if tmp == "" {
		return "", fmt.Errorf("no runtime dir available for the mpv socket")
	}
	return filepath.Join(tmp, fmt.Sprintf("streambeats-mpv-%d.sock", os.Getuid())), nil
}

// mpvArgs is the headless mpv command line: no window, no terminal input/mux,
// keep running while idle, and listen on our IPC socket.
func mpvArgs(socketPath string) []string {
	return []string{
		"--no-video",
		"--no-terminal",
		"--idle",
		"--input-ipc-server=" + socketPath,
	}
}

// SpawnMPV starts mpv in the background and dials its IPC socket. The caller
// owns the returned command/client and must stop them on exit.
func SpawnMPV(socketPath string) (*exec.Cmd, *player.Client, error) {
	bin, err := exec.LookPath("mpv")
	if err != nil {
		return nil, nil, fmt.Errorf("mpv not found in PATH: %w", err)
	}

	// Spawn the pinned mpv path that LookPath resolved; args come strictly
	// from our own options (no user input), so inline #nosec exempts G204.
	cmd := exec.CommandContext(context.Background(), bin, mpvArgs(socketPath)...) // #nosec G204 — pinned binary, internal args
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start mpv: %w", err)
	}
	client, err := dialMPV(socketPath, mpvStartupTimeout)
	if err != nil {
		stopProcess(cmd)
		return nil, nil, err
	}
	return cmd, client, nil
}

// dialMPV retries connecting until the socket appears or the timeout expires.
func dialMPV(socketPath string, timeout time.Duration) (*player.Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		client, err := player.Dial(socketPath, mpvDialTimeout)
		if err == nil {
			return client, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("connect to mpv ipc: %w", lastErr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// stopProcess terminates and reaps a spawned child process (best effort).
func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
