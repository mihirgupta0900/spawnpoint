package ui

import (
	"errors"
	"os"
	"sync/atomic"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/term"
)

// Interactive reports whether stderr is a terminal (so spinners and pickers
// make sense).
func Interactive() bool { return term.IsTerminal(os.Stderr.Fd()) }

type spinDone struct{}

type spinModel struct {
	spin  spinner.Model
	title string
	run   func()
	done  bool
}

func (m *spinModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, func() tea.Msg { m.run(); return spinDone{} })
}

func (m *spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinDone:
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Interrupt
		}
	}
	var cmd tea.Cmd
	m.spin, cmd = m.spin.Update(msg)
	return m, cmd
}

// View clears itself once the action finishes, so the next line of output
// starts clean.
func (m *spinModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.spin.View() + " " + dimStyle.Render(m.title))
}

// Spin runs action behind a spinner titled title. Without a terminal it just
// runs the action.
func Spin(title string, action func()) {
	if !Interactive() {
		action()
		return
	}
	var started atomic.Bool
	m := &spinModel{
		spin:  spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(Purple))),
		title: title,
		run:   func() { started.Store(true); action() },
	}
	_, err := runInline(m)
	switch {
	case errors.Is(err, tea.ErrInterrupted):
		Println(Dim("Interrupted."))
		os.Exit(130)
	case err != nil && !started.Load():
		action() // the terminal couldn't host the spinner; do the work anyway
	}
}

// Table renders a rounded, dim-bordered table with a bold header row.
func Table(headers []string, rows [][]string) {
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(Faint)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(Blue)
			}
			return s
		})
	Println(t.Render())
}
