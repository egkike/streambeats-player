package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Terminal-adaptive transport (issue #64): the Solo Teclado keycaps are
// calibrated for ghostty 1.3.1 (legacy grapheme-width method), so the emoji
// glyphs only close their frames there. Everywhere else the row must render
// pure-ASCII tokens whose widths are stable by rune count — the same doctrine
// as the pre-VS16 rounds. These tests pin the non-ghostty path; the calibrated
// ghostty path stays guarded by the existing view_test.go pins.

// S7: detection fails closed — only the exact value "ghostty" selects the
// calibrated keycaps; unset, empty or any foreign value selects ASCII.
func TestGhosttyDetectionFailsClosed(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"ghostty", true},
		{"", false},
		{"kitty", false},
		{"xterm-256color", false},
		{"GHOSTTY", false},
		{" ghostty", false},
	}
	for _, tc := range cases {
		if got := isGhosttyTerminal(tc.env); got != tc.want {
			t.Fatalf("isGhosttyTerminal(%q) = %v, want %v (fails closed to ASCII)", tc.env, got, tc.want)
		}
	}
}

// S6: the ASCII token set carries no emoji, no VS16, nothing outside printable
// ASCII — so every width equals the token's rune count on every terminal.
func TestASCIITokensArePlainAndWidthStable(t *testing.T) {
	if len(asciiTokens) != len(keycaps) {
		t.Fatalf("asciiTokens = %d entries, want one per calibrated cap (%d)", len(asciiTokens), len(keycaps))
	}
	for _, c := range keycaps {
		tok, ok := asciiTokens[c.icon]
		if !ok {
			t.Fatalf("calibrated icon %q has no ASCII token", c.icon)
		}
		if utf8.RuneCountInString(tok) != len(tok) {
			t.Fatalf("ASCII token %q for %q must be single-byte ASCII (width = rune count)", tok, c.icon)
		}
		for _, r := range tok {
			if r < 0x20 || r >= 0x7f {
				t.Fatalf("ASCII token %q for %q contains non-printable-ASCII rune %q", tok, c.icon, r)
			}
		}
	}
}

// S2/S6/S7: with AsciiTransport set (what Launch does whenever TERM_PROGRAM
// is not exactly "ghostty"), the keycap row renders the ASCII tokens with
// hand-built borders that CLOSE along the rune-count widths: interiors
// 6/8/6/6/9, frames 8/10/8/8/11 wide end to end.
func TestKeycapsRowRendersASCIIOutsideGhostty(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	m := layoutModel("")
	m.width, m.height = 52, 24
	m.asciiTransport = true
	rows := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
	caps := stripANSI(strings.Join(rows[20:23], "\n"))
	for _, want := range []string{
		"╭──────╮╭────────╮╭──────╮╭──────╮╭─────────╮",
		"│ |< p ││ || spc ││ >| n ││ [] s ││ vol +/- │",
		"╰──────╯╰────────╯╰──────╯╰──────╯╰─────────╯",
	} {
		if !strings.Contains(caps, want) {
			t.Fatalf("ASCII keycaps missing %q:\n%s", want, caps)
		}
	}
	for _, ghosttyGlyph := range []string{"⏮️", "⏯️", "⏹️", "⏭️", "🔊", "\ufe0f"} {
		if strings.Contains(caps, ghosttyGlyph) {
			t.Fatalf("ASCII transport leaked the ghostty glyph %q:\n%s", ghosttyGlyph, caps)
		}
	}
}

// S7 (fail closed at the public entry): a model whose Options say
// AsciiTransport renders ASCII — the zero value stays calibrated (S5: the
// existing pins hold byte-identically on the ghostty path).
func TestOptionsWireTheTransportMode(t *testing.T) {
	if _, ok := asciiTokens[keycaps[0].icon]; !ok {
		t.Fatalf("the token table must cover every calibrated cap")
	}
	m := NewModel(Options{Catalog: nil, AsciiTransport: true})
	if !m.asciiTransport {
		t.Fatalf("Options.AsciiTransport = true must reach the model")
	}
	if (NewModel(Options{})).asciiTransport {
		t.Fatalf("zero-value Options must keep the calibrated ghostty path (S5)")
	}
}
