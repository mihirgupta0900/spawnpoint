package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func press(m *picker, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		default: // literal text
			for _, r := range k {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		m.Update(msg)
	}
}

var repos = []string{"api", "billing-docs", "infra", "web", "worker"}

func TestPickerFuzzyFilter(t *testing.T) {
	m := newPicker(PickOptions{Items: repos, Multi: true})
	press(m, "wr")
	var got []string
	for _, mt := range m.matches {
		got = append(got, repos[mt.idx])
	}
	if !slices.Equal(got, []string{"worker"}) {
		t.Fatalf("matches = %v", got)
	}
}

func TestPickerToggleClearsSearch(t *testing.T) {
	m := newPicker(PickOptions{Items: repos, Multi: true})
	press(m, "web", "space")
	if m.query != "" || len(m.matches) != len(repos) {
		t.Fatalf("search not cleared: query=%q matches=%d", m.query, len(m.matches))
	}
	if repos[m.matches[m.cursor].idx] != "web" {
		t.Fatalf("cursor on %s, want web", repos[m.matches[m.cursor].idx])
	}
	press(m, "api", "tab", "enter")
	if !m.done || !slices.Equal(m.result(), []string{"api", "web"}) {
		t.Fatalf("result = %v (done=%v)", m.result(), m.done)
	}
	if v := m.View().Content; v != "" {
		t.Fatalf("view not cleared after confirm: %q", v)
	}
}

func TestPickerEnterPicksHighlightedWhenNothingToggled(t *testing.T) {
	m := newPicker(PickOptions{Items: repos, Multi: true})
	press(m, "down", "down", "enter")
	if !slices.Equal(m.result(), []string{"infra"}) {
		t.Fatalf("result = %v", m.result())
	}
}

func TestPickerPreselectedAndSelectedLine(t *testing.T) {
	m := newPicker(PickOptions{Items: repos, Selected: []string{"worker", "api"}, Multi: true})
	if !slices.Equal(m.result(), []string{"api", "worker"}) {
		t.Fatalf("result = %v", m.result())
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "selected (2): api, worker") {
		t.Fatalf("view:\n%s", m.View().Content)
	}
}

func TestPickerSingleSelectTypesSpaces(t *testing.T) {
	m := newPicker(PickOptions{Items: []string{"feat-a  (1 repo)", "feat-b  (2 repos)"}})
	press(m, "b", "space", "2", "enter")
	if !slices.Equal(m.result(), []string{"feat-b  (2 repos)"}) {
		t.Fatalf("result = %v", m.result())
	}
}

func TestPickerNoMatchesDoesNotConfirm(t *testing.T) {
	m := newPicker(PickOptions{Items: repos, Multi: true})
	press(m, "zzz", "enter")
	if m.done {
		t.Fatal("confirmed with nothing to pick")
	}
	press(m, "backspace", "backspace", "backspace", "esc")
	if !m.aborted {
		t.Fatal("esc should abort")
	}
}
