package ui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// ErrAborted is returned when the user cancels a prompt (ctrl+c / esc).
var ErrAborted = errors.New("aborted")

func theme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeCharm(isDark)
		t.Focused.Title = t.Focused.Title.Foreground(Blue)
		t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(Purple)
		t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(Purple)
		t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(Purple)
		t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(Blue)
		t.Focused.FocusedButton = t.Focused.FocusedButton.Background(Blue).Foreground(lipgloss.Color("#0A0D14"))
		t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(Green)
		t.Blurred = t.Focused
		t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
		return t
	})
}

// formModel hosts a huh form in our own program (see runInline) so its
// frame can be cleared once answered.
type formModel struct{ f *huh.Form }

func (m formModel) Init() tea.Cmd { return m.f.Init() }

func (m formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	f, cmd := m.f.Update(msg)
	m.f = f.(*huh.Form)
	return m, cmd
}

func (m formModel) View() tea.View { return tea.NewView(m.f.View()) }

func run(field huh.Field) error {
	f := huh.NewForm(huh.NewGroup(field)).WithTheme(theme()).WithShowHelp(false)
	f.SubmitCmd = tea.Quit
	f.CancelCmd = tea.Interrupt
	_, err := runInline(formModel{f})
	if errors.Is(err, tea.ErrInterrupted) || f.State == huh.StateAborted {
		return ErrAborted
	}
	return err
}

// answered leaves a one-line record of a finished prompt, since huh clears
// its view on completion.
func answered(title, answer string) {
	Println(Good("✓") + " " + Bold(strings.TrimSuffix(title, ":")) + " " + Accent(answer))
}

// Confirm asks a yes/no question.
func Confirm(title string, def bool) (bool, error) {
	v := def
	if err := run(huh.NewConfirm().Title(title).Affirmative("Yes").Negative("No").Value(&v)); err != nil {
		return false, err
	}
	answer := "No"
	if v {
		answer = "Yes"
	}
	answered(title, answer)
	return v, nil
}

// Input asks for a line of text.
func Input(title, def string) (string, error) {
	v := def
	if err := run(huh.NewInput().Title(title).Value(&v)); err != nil {
		return "", err
	}
	v = strings.TrimSpace(v)
	answered(title, v)
	return v, nil
}

// Select asks the user to choose one of options.
func Select(title string, options []string, def string) (string, error) {
	v := def
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	if err := run(huh.NewSelect[string]().Title(title).Options(opts...).Value(&v)); err != nil {
		return "", err
	}
	answered(title, v)
	return v, nil
}

// SelectLabeled is Select with display labels distinct from the returned
// values.
func SelectLabeled(title string, labels, values []string, def string) (string, error) {
	v := def
	opts := make([]huh.Option[string], len(labels))
	for i := range labels {
		opts[i] = huh.NewOption(labels[i], values[i])
	}
	if err := run(huh.NewSelect[string]().Title(title).Options(opts...).Value(&v)); err != nil {
		return "", err
	}
	for i := range values {
		if values[i] == v {
			answered(title, labels[i])
		}
	}
	return v, nil
}

// Checklist is a plain multi-select (no fuzzy search) with every option
// pre-selected.
func Checklist(title string, labels, values []string) ([]string, error) {
	v := append([]string(nil), values...)
	opts := make([]huh.Option[string], len(labels))
	for i := range labels {
		opts[i] = huh.NewOption(labels[i], values[i]).Selected(true)
	}
	if err := run(huh.NewMultiSelect[string]().Title(title).Options(opts...).Value(&v)); err != nil {
		return nil, err
	}
	answered(title, strings.Join(v, ", "))
	return v, nil
}
