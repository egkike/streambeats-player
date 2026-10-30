package tui

// Detection tests for the panel-terminal choice in the settings screen
// (issue #90, S3/S7). The lookup is injected so no test touches the real
// PATH (project standard: no environment-dependent assertions); detection
// fails closed — a terminal missing from PATH is simply absent from the
// offered rows, never guessed or substituted.

import (
	"os/exec"
	"strings"
	"testing"
)

// lookupFor builds an injected lookup: only the listed binaries resolve.
func lookupFor(installed ...string) func(string) (string, error) {
	ok := make(map[string]struct{}, len(installed))
	for _, bin := range installed {
		ok[bin] = struct{}{}
	}
	return func(bin string) (string, error) {
		if _, hit := ok[bin]; hit {
			return "/fake/" + bin, nil
		}
		return "", exec.ErrNotFound
	}
}

// S3 canonical order: rows always come out ghostty first, kitty second,
// regardless of which lookup hits.
func TestPanelTerminalRowsCanonicalOrder(t *testing.T) {
	both := panelTerminalRows(lookupFor("kitty", "ghostty"))
	if strings.Join(both, ",") != "ghostty,kitty" {
		t.Errorf("both installed = %v, want [ghostty kitty] in canonical order", both)
	}
	kittyOnly := panelTerminalRows(lookupFor("kitty"))
	if strings.Join(kittyOnly, ",") != "kitty" {
		t.Errorf("kitty-only installed = %v, want [kitty]", kittyOnly)
	}
}

// S3/S7 fail closed: a missing terminal never appears as a selectable row.
func TestPanelTerminalRowsExcludeMissing(t *testing.T) {
	ghosttyOnly := panelTerminalRows(lookupFor("ghostty"))
	if strings.Join(ghosttyOnly, ",") != "ghostty" {
		t.Errorf("ghostty-only installed = %v, want [ghostty]; missing rows must be excluded", ghosttyOnly)
	}
}

// S7: with neither terminal installed the selection list is empty (the
// section renders its note; the launcher keeps today's friendly error).
func TestPanelTerminalRowsEmptyWhenNothingInstalled(t *testing.T) {
	none := panelTerminalRows(lookupFor())
	if len(none) != 0 {
		t.Errorf("nothing installed = %v, want no rows (fail closed, friendly error path)", none)
	}
}

// Detection never invents choices: an installed binary outside the offer set
// must not surface as a row.
func TestPanelTerminalRowsIgnoreForeignBinaries(t *testing.T) {
	got := panelTerminalRows(lookupFor("bash", "zsh"))
	if len(got) != 0 {
		t.Errorf("foreign installed binaries = %v, want no rows (no invented choices)", got)
	}
}

// S3: the current persisted choice is selectable when installed and its row
// is recognizable for the (current) marker; validity check rejects unknown
// ids before persistence can store them.
func TestPanelTerminalValidatedAgainstOffer(t *testing.T) {
	valid := []string{"ghostty", "kitty"}
	for _, id := range valid {
		if !panelTerminalValid(id) {
			t.Errorf("panelTerminalValid(%q) = false, want true (canonical choice)", id)
		}
	}
	for _, bad := range []string{"", "wezterm", "alacritty", "GHOSTTY", " kitty"} {
		if panelTerminalValid(bad) {
			t.Errorf("panelTerminalValid(%q) = true, want false (fails closed)", bad)
		}
	}
}
