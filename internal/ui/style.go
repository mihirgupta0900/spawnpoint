// Package ui holds everything the user sees: styled status output, prompts,
// the fuzzy picker, spinners and tables.
//
// All human-readable output goes to stderr so stdout stays reserved for
// machine output (--json, or the bare workspace path in --no-input mode).
package ui

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
)

// Out is where human-readable output goes.
var Out io.Writer = os.Stderr

var (
	Blue   = lipgloss.Color("#58A6FF")
	Purple = lipgloss.Color("#BC8CFF")
	Green  = lipgloss.Color("#3FB950")
	Yellow = lipgloss.Color("#E3B341")
	Red    = lipgloss.Color("#F85149")
	Orange = lipgloss.Color("#F0883E")
	Gray   = lipgloss.Color("#8B95A5")
	Faint  = lipgloss.Color("#5A6474")
)

var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(Gray)
	faintStyle  = lipgloss.NewStyle().Foreground(Faint)
	blueStyle   = lipgloss.NewStyle().Foreground(Blue)
	purpleStyle = lipgloss.NewStyle().Foreground(Purple)
	greenStyle  = lipgloss.NewStyle().Foreground(Green)
	yellowStyle = lipgloss.NewStyle().Foreground(Yellow)
	redStyle    = lipgloss.NewStyle().Foreground(Red)
	headerStyle = lipgloss.NewStyle().Foreground(Blue).Bold(true)
)

func Bold(s string) string   { return boldStyle.Render(s) }
func Dim(s string) string    { return dimStyle.Render(s) }
func Faded(s string) string  { return faintStyle.Render(s) }
func Accent(s string) string { return blueStyle.Render(s) }
func Accent2(s string) string {
	return purpleStyle.Render(s)
}
func Good(s string) string { return greenStyle.Render(s) }
func Warn(s string) string { return yellowStyle.Render(s) }
func Bad(s string) string  { return redStyle.Render(s) }

// Println writes a line of (possibly styled) output, downsampling colors to
// what the terminal supports and stripping them when it isn't a TTY.
func Println(a ...any) { lipgloss.Fprintln(Out, a...) }

func Printf(format string, a ...any) { lipgloss.Fprint(Out, fmt.Sprintf(format, a...)) }

// Blank prints an empty line.
func Blank() { lipgloss.Fprintln(Out) }

// Header is a section heading, e.g. "Preparing repositories".
func Header(s string) { Println(headerStyle.Render(s)) }

// Success is a green check line.
func Success(format string, a ...any) {
	Println(greenStyle.Render("✓") + " " + fmt.Sprintf(format, a...))
}

// Warning is a yellow line.
func Warning(format string, a ...any) {
	Println(yellowStyle.Render("! " + fmt.Sprintf(format, a...)))
}

// Error prints "✗ Error: ..." in red.
func Error(format string, a ...any) {
	Println(redStyle.Bold(true).Render("✗ Error:") + " " + fmt.Sprintf(format, a...))
}

// Hint is an indented dim line.
func Hint(format string, a ...any) {
	Println(dimStyle.Render("  " + fmt.Sprintf(format, a...)))
}

// Item is an indented line with a dim bullet.
func Item(format string, a ...any) {
	Println("  " + faintStyle.Render("•") + " " + fmt.Sprintf(format, a...))
}

// Done prints the final "Done!" banner.
func Done(extra string) {
	Blank()
	line := greenStyle.Bold(true).Render("✓ Done!")
	if extra != "" {
		line += " " + extra
	}
	Println(line)
}
