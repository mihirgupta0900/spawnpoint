package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
)

// PickOptions configures Pick.
type PickOptions struct {
	Title    string
	Items    []string // display labels, returned as-is
	Selected []string // labels toggled on at start (multi only)
	Multi    bool
}

// Pick shows a type-to-search fuzzy picker and returns the chosen labels in
// list order. Typing filters immediately; tab/space toggles in multi mode and
// clears the search so the next repo can be found; enter confirms (picking
// the highlighted row if nothing is toggled).
func Pick(opts PickOptions) ([]string, error) {
	pm := newPicker(opts)
	if _, err := runInline(pm); err != nil {
		return nil, err
	}
	if pm.aborted {
		return nil, ErrAborted
	}
	result := pm.result()
	answered(opts.Title, strings.Join(result, ", "))
	return result, nil
}

const pickerHeight = 10

type match struct {
	idx       int
	positions []int
}

type picker struct {
	opts     PickOptions
	query    string
	matches  []match
	cursor   int
	offset   int
	selected map[int]bool
	done     bool
	aborted  bool
}

func newPicker(opts PickOptions) *picker {
	m := &picker{opts: opts, selected: map[int]bool{}}
	pre := map[string]bool{}
	for _, s := range opts.Selected {
		pre[s] = true
	}
	for i, it := range opts.Items {
		if pre[it] {
			m.selected[i] = true
		}
	}
	m.filter()
	return m
}

func (m *picker) Init() tea.Cmd { return nil }

func (m *picker) filter() {
	m.matches = m.matches[:0]
	if m.query == "" {
		for i := range m.opts.Items {
			m.matches = append(m.matches, match{idx: i})
		}
	} else {
		for _, r := range fuzzy.Find(m.query, m.opts.Items) {
			m.matches = append(m.matches, match{idx: r.Index, positions: r.MatchedIndexes})
		}
	}
	m.cursor = min(m.cursor, max(len(m.matches)-1, 0))
	m.scroll()
}

func (m *picker) scroll() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+pickerHeight {
		m.offset = m.cursor - pickerHeight + 1
	}
}

func (m *picker) move(d int) {
	if len(m.matches) == 0 {
		return
	}
	m.cursor = (m.cursor + d + len(m.matches)) % len(m.matches)
	m.scroll()
}

func (m *picker) toggle() {
	if len(m.matches) == 0 {
		return
	}
	idx := m.matches[m.cursor].idx
	m.selected[idx] = !m.selected[idx]
	if m.query != "" {
		// Clear the search and keep the cursor on the row just toggled.
		m.query = ""
		m.filter()
		m.cursor = idx
		m.scroll()
	}
}

func (m *picker) result() []string {
	var out []string
	if m.opts.Multi {
		for i, it := range m.opts.Items {
			if m.selected[i] {
				out = append(out, it)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if len(m.matches) > 0 {
		return []string{m.opts.Items[m.matches[m.cursor].idx]}
	}
	return nil
}

func (m *picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc":
		m.aborted = true
		return m, tea.Quit
	case "enter":
		if len(m.result()) == 0 {
			return m, nil
		}
		m.done = true
		return m, tea.Quit
	case "up", "ctrl+p", "ctrl+k":
		m.move(-1)
	case "down", "ctrl+n", "ctrl+j":
		m.move(1)
	case "pgup":
		m.move(-pickerHeight)
	case "pgdown":
		m.move(pickerHeight)
	case "tab", "space":
		if m.opts.Multi {
			m.toggle()
			if key.String() == "tab" {
				m.move(1)
			}
		} else if key.String() == "space" {
			m.query += " "
			m.filter()
		}
	case "ctrl+a":
		if m.opts.Multi {
			all := true
			for _, mt := range m.matches {
				all = all && m.selected[mt.idx]
			}
			for _, mt := range m.matches {
				m.selected[mt.idx] = !all
			}
		}
	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.filter()
		}
	case "ctrl+u", "ctrl+w":
		m.query = ""
		m.filter()
	default:
		if key.Text != "" {
			m.query += key.Text
			m.cursor = 0
			m.filter()
		}
	}
	return m, nil
}

var (
	pTitle    = lipgloss.NewStyle().Foreground(Blue).Bold(true)
	pPrompt   = lipgloss.NewStyle().Foreground(Purple)
	pCursor   = lipgloss.NewStyle().Foreground(Purple).Bold(true)
	pRowOn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3"))
	pRow      = lipgloss.NewStyle().Foreground(Gray)
	pMatch    = lipgloss.NewStyle().Foreground(Blue).Bold(true)
	pChecked  = lipgloss.NewStyle().Foreground(Green)
	pDim      = lipgloss.NewStyle().Foreground(Faint)
	pSelected = lipgloss.NewStyle().Foreground(Green)
)

func (m *picker) View() tea.View {
	if m.done || m.aborted {
		return tea.NewView("")
	}
	var b strings.Builder

	help := "type to search · ↑↓ move · enter confirm"
	if m.opts.Multi {
		help = "type to search · tab/space toggle · ctrl+a all · enter confirm"
	}
	b.WriteString(pTitle.Render(m.opts.Title) + "  " + pDim.Render(help) + "\n")

	count := pDim.Render(fmt.Sprintf("%d/%d", len(m.matches), len(m.opts.Items)))
	line := pPrompt.Render("› ") + m.query + pCursor.Render("▏") + "  " + count
	if m.opts.Multi {
		var names []string
		for i, it := range m.opts.Items {
			if m.selected[i] {
				names = append(names, it)
			}
		}
		if len(names) > 0 {
			line += pDim.Render("  ·  ") + pSelected.Render(fmt.Sprintf("selected (%d): ", len(names))) + strings.Join(names, ", ")
		}
	}
	b.WriteString(line + "\n")

	// The frame keeps a constant height (rows are padded) so filtering never
	// shrinks it: Bubble Tea's inline renderer leaves stale lines behind when
	// a frame gets shorter.
	rows := min(len(m.opts.Items), pickerHeight)
	end := min(m.offset+pickerHeight, len(m.matches))
	drawn := 0
	for i := m.offset; i < end; i++ {
		drawn++
		mt := m.matches[i]
		label := m.opts.Items[mt.idx]
		here := i == m.cursor

		cur := "  "
		if here {
			cur = pCursor.Render("❯ ")
		}
		box := ""
		if m.opts.Multi {
			if m.selected[mt.idx] {
				box = pChecked.Render("◉ ")
			} else {
				box = pDim.Render("○ ")
			}
		}
		base := pRow
		if here || m.selected[mt.idx] {
			base = pRowOn
		}
		b.WriteString(cur + box + highlight(label, mt.positions, base) + "\n")
	}
	if len(m.matches) == 0 {
		b.WriteString(pDim.Render("  no matches") + "\n")
		drawn++
	}
	for ; drawn < rows; drawn++ {
		b.WriteString("\n")
	}
	if len(m.opts.Items) > pickerHeight {
		if hidden := len(m.matches) - end; hidden > 0 {
			b.WriteString(pDim.Render(fmt.Sprintf("  … %d more", hidden)))
		}
		b.WriteString("\n")
	}
	return tea.NewView(b.String())
}

func highlight(s string, positions []int, base lipgloss.Style) string {
	if len(positions) == 0 {
		return base.Render(s)
	}
	hit := map[int]bool{}
	for _, p := range positions {
		hit[p] = true
	}
	// Render runs of matched / unmatched runes (positions are byte offsets).
	var b strings.Builder
	var run strings.Builder
	runHit := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if runHit {
			b.WriteString(pMatch.Render(run.String()))
		} else {
			b.WriteString(base.Render(run.String()))
		}
		run.Reset()
	}
	for i, r := range s {
		if hit[i] != runHit {
			flush()
			runHit = hit[i]
		}
		run.WriteRune(r)
	}
	flush()
	return b.String()
}
