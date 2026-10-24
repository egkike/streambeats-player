package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestZZDebugAdvance2(t *testing.T) {
	m := testModel(Options{
		Player:       &fakePlayer{},
		LibraryFiles: []string{"/lib/a1.mp3", "/lib/a2.mp3"},
	})
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	t.Logf("mode after 'l' = %v", m.mode)
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	t.Logf("cmd after enter=%v", cmd != nil)
	msg := runCmd(t, cmd)
	m, _ = update(t, m, msg)
	t.Logf("playingLib=%d playing=%v", m.playingLib, m.playing)
	var nc tea.Cmd
	for i := 0; i <= progressErrorThreshold; i++ {
		m, nc = update(t, m, progressMsg{err: errors.New("x")})
		t.Logf("i=%d misses=%d cmd=%v mode=%v", i, m.progressMisses, nc != nil, m.mode)
	}
}
