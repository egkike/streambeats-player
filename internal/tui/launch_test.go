package tui

import (
	"testing"
)

// The diagnostic input kit must degrade gracefully when stdin is not a TTY
// (tests, systemd redirections): logging still works, raw-mode is skipped
// without panic, and no state leaks.
func TestDebugInputLogDegradesWithoutTTY(t *testing.T) {
	t.Setenv("STREAMBEATS_DEBUG_INPUT", "1")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	f, r, logf := debugInputLog()
	if f == nil || r == nil || logf == nil {
		t.Fatalf("kit must provide log/reader/hook even without a TTY (stdin may be non-tty under tests)")
	}
	logf("probe line")
	st := rawStdinState()
	if st != nil {
		t.Skipf("stdin IS a TTY in this environment; the raw branch ran")
	}
	_ = f.Close()
}
