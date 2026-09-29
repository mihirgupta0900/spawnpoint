// Package cli wires the spawnpoint commands together with cobra and fang.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/update"
)

// exitError ends the command with a status code after the command has
// already explained what went wrong.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func exit(code int) error { return exitError{code} }

// fail prints an error line (plus optional hint lines) and exits 1.
func fail(format string, a ...any) error {
	ui.Error(format, a...)
	return exit(1)
}

// Execute runs the CLI and returns the process exit code.
func Execute(version string) int {
	root := newRoot()
	checker := update.NewChecker(version)
	root.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		level := slog.LevelWarn
		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		if skipUpdateCheck(cmd) {
			return
		}
		if cfg, err := config.Load(); err == nil && cfg.CheckUpdates {
			checker.Start()
		}
	}

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(version),
		fang.WithColorSchemeFunc(colorScheme),
		fang.WithErrorHandler(errorHandler),
	)
	if notice := checker.Notice(); notice != "" {
		ui.Println(notice)
	}

	var ee exitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.code
	case errors.Is(err, ui.ErrAborted):
		return 130
	default:
		return 1
	}
}

// skipUpdateCheck keeps agents' output clean and avoids checking during
// `update` itself.
func skipUpdateCheck(cmd *cobra.Command) bool {
	if cmd.Name() == "update" || cmd.Name() == "completion" || cmd.Name() == "man" {
		return true
	}
	for _, name := range []string{"json", "no-input"} {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			return true
		}
	}
	return false
}

func errorHandler(w io.Writer, styles fang.Styles, err error) {
	var ee exitError
	switch {
	case errors.As(err, &ee):
		return
	case errors.Is(err, ui.ErrAborted):
		ui.Println(ui.Dim("Aborted."))
		return
	}
	fang.DefaultErrorHandler(w, styles, err)
}

func colorScheme(ld lipgloss.LightDarkFunc) fang.ColorScheme {
	cs := fang.DefaultColorScheme(ld)
	cs.Title = ui.Blue
	cs.Command = ui.Purple
	cs.Program = ui.Blue
	cs.Flag = ui.Green
	cs.Argument = ld(lipgloss.Color("#1F2328"), lipgloss.Color("#E6EDF3"))
	cs.DimmedArgument = ui.Gray
	cs.Comment = ui.Gray
	cs.QuotedString = ui.Yellow
	cs.ErrorHeader = [2]color.Color{lipgloss.Color("#FFFFFF"), ui.Red}
	return cs
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "spawnpoint",
		Short: "One branch. Every repo. Ready to code.",
		Long: "Spawnpoint gives every task its own folder: a git worktree for each repo it\n" +
			"touches, all on the same branch, with .env files copied and dependencies\n" +
			"installed. Point Claude Code, Codex, or yourself at it and start working.",
		Example: strings.Join([]string{
			"# pick repos, name a branch, spawn",
			"sp create",
			"",
			"# from a script or agent",
			"spawnpoint create --no-input --json --repos api,web --branch feat/billing",
		}, "\n"),
		SilenceUsage: true,
	}
	root.PersistentFlags().Bool("debug", false, "enable debug logging")

	root.AddGroup(
		&cobra.Group{ID: "work", Title: "Workspaces"},
		&cobra.Group{ID: "setup", Title: "Setup"},
	)
	for _, c := range []*cobra.Command{
		createCmd(), addCmd(), listCmd(), reposCmd(), templateCmd(), cleanupCmd(), lightCleanupCmd(),
	} {
		c.GroupID = "work"
		root.AddCommand(c)
	}
	for _, c := range []*cobra.Command{initCmd(), configCmd(), updateCmd()} {
		c.GroupID = "setup"
		root.AddCommand(c)
	}
	root.SetHelpCommandGroupID("setup")
	root.SetCompletionCommandGroupID("setup")
	cobra.EnableCommandSorting = false
	return root
}

// agentFlags are shared by every command that can run headless.
type agentFlags struct {
	noInput bool
	json    bool
}

func (a *agentFlags) register(cmd *cobra.Command, noInputHelp string) {
	if noInputHelp != "" {
		cmd.Flags().BoolVarP(&a.noInput, "no-input", "n", false, noInputHelp)
	}
	cmd.Flags().BoolVar(&a.json, "json", false, "emit machine-readable JSON to stdout")
}

// emit prints v as indented JSON on stdout.
func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// ensureConfig loads the config, running first-time setup if none exists.
// Headless callers get a config written from detected defaults instead of a
// prompt, so agents never hang.
func ensureConfig(a agentFlags) (*config.Config, error) {
	if !config.Exists() {
		if a.noInput || a.json {
			cfg := config.Default()
			cfg.ScanDirs = config.DetectScanDirs()
			if _, err := config.Save(cfg); err != nil {
				return nil, err
			}
			return config.Load()
		}
		ui.Println(ui.Bold("Welcome to Spawnpoint!") + " Let's set things up.")
		ui.Blank()
		if err := runInit(); err != nil {
			return nil, err
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fail("%v", err)
	}
	return cfg, nil
}

// parseCSV splits a comma-separated flag into trimmed, non-empty parts.
func parseCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func requireFlag(value, flag string) error {
	if strings.TrimSpace(value) == "" {
		return fail("--no-input requires %s.", flag)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func orNone(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

// tilde shortens paths under $HOME for display.
func tilde(p string) string {
	if h, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, h+string(os.PathSeparator)) {
		return "~" + p[len(h):]
	}
	return p
}
