package tui

// THROWAWAY diagnostic (not for commit): feed an SGR click through the real
// tea input pipeline and observe the final model state (oracle: the toggle
// click must surface the same status the space keybind produces).
import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type delayedReader struct {
	seq  []byte
	pos  int
	step time.Duration
	t0   time.Time
}

func (r *delayedReader) Read(p []byte) (int, error) {
	// deliver one byte every 60ms then EOF after q
	now := time.Since(r.t0)
	idx := int(now / r.step)
	if idx >= len(r.seq) {
		return 0, io.EOF
	}
	if r.pos < idx {
		r.pos = idx
	}
	if r.pos >= len(r.seq) {
		return 0, io.EOF
	}
	n := copy(p, r.seq[r.pos:r.pos+1])
	r.pos++
	return n, nil
}

func TestFeedMouseThroughTeaPipeline(t *testing.T) {
	seq := "\x1b[<0;5;21;M\x1b[<0;5;21;m" + "q"
	fp := &fakePlayer{}
	m := NewModel(Options{Player: fp})
	m.width, m.height = 52, 24

	p := tea.NewProgram(m,
		tea.WithInput(&delayedReader{seq: []byte(seq), step: 60 * time.Millisecond, t0: time.Now()}),
		tea.WithoutRenderer(),
		tea.WithOutput(io.Discard),
	)
	done := make(chan struct{}, 1)
	var finalModel tea.Model
	go func() {
		finalModel, _ = p.Run()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
	}
	if fm, ok := finalModel.(Model); ok {
		t.Logf("final status: %q | pauseCalls=%d", fm.status, fp.pauseCalls)
	} else {
		t.Fatalf("no final model (run did not complete); pauseCalls=%d", fp.pauseCalls)
	}
}
