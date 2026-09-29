package ui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// inline wraps a model and remembers how tall its last frame was.
//
// Bubble Tea v2's inline renderer leaves a finished program's last frame on
// screen, which would stack stale pickers and prompts above our one-line
// summaries. runInline erases the frame after the program exits instead.
type inline struct {
	inner  tea.Model
	height int
}

func (m *inline) Init() tea.Cmd { return m.inner.Init() }

func (m *inline) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

func (m *inline) View() tea.View {
	v := m.inner.View()
	// Drop only the final newline: padded blank lines before it are part of
	// the frame.
	if c := strings.TrimSuffix(v.Content, "\n"); c != "" {
		m.height = strings.Count(c, "\n") + 1
		v.SetContent(c)
	}
	return v
}

// runInline runs model on stderr and clears its output when it finishes.
func runInline(model tea.Model) (tea.Model, error) {
	w := &inline{inner: model}
	_, err := tea.NewProgram(w, tea.WithOutput(os.Stderr)).Run()
	if w.height > 1 {
		fmt.Fprintf(os.Stderr, "\x1b[%dA", w.height-1)
	}
	fmt.Fprint(os.Stderr, "\r\x1b[J")
	return w.inner, err
}
