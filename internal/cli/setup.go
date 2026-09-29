package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup: scan dirs, workspace location, shell integration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if config.Exists() {
				ok, err := ui.Confirm(fmt.Sprintf("Config already exists at %s. Overwrite?", tilde(config.Path())), false)
				if err != nil {
					return err
				}
				if !ok {
					ui.Println("Keeping existing config.")
					return offerShellIntegration()
				}
			}
			return runInit()
		},
	}
}

func runInit() error {
	cfg := config.Default()

	if detected := config.DetectScanDirs(); len(detected) > 0 {
		ui.Println("Found these code directories:")
		for _, d := range detected {
			ui.Println("  " + ui.Good("✓") + " " + tilde(d))
		}
		ui.Blank()
		ok, err := ui.Confirm("Use these as scan directories?", true)
		if err != nil {
			return err
		}
		if ok {
			cfg.ScanDirs = detected
		}
	}

	extras, err := ui.Input("Add any other directories? (comma-separated, enter to skip)", "")
	if err != nil {
		return err
	}
	for _, d := range parseCSV(extras) {
		p := config.ExpandPath(d)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			ui.Warning("Skipping %s (does not exist)", d)
			continue
		}
		if !slices.Contains(cfg.ScanDirs, p) {
			cfg.ScanDirs = append(cfg.ScanDirs, p)
		}
	}
	if len(cfg.ScanDirs) == 0 {
		ui.Warning("No scan directories set. You can add them later in the config.")
	}

	wt, err := ui.Input("Where should workspaces be created?", tilde(filepath.Join(config.Dir(), "workspaces")))
	if err != nil {
		return err
	}
	cfg.WorktreeDir = config.ExpandPath(wt)

	if cfg.AutoInstallDeps, err = ui.Confirm("Auto-install dependencies after creating worktrees?", true); err != nil {
		return err
	}

	path, err := config.Save(cfg)
	if err != nil {
		return fail("saving config: %v", err)
	}
	ui.Blank()
	ui.Success("Config saved to %s", tilde(path))
	ui.Hint("Run %s to view or edit it later.", ui.Bold("spawnpoint config"))
	ui.Blank()
	return offerShellIntegration()
}

const shellMarker = "spawnpoint shell integration"

const posixSnippet = `
# spawnpoint shell integration
sp() {
    local cmd="${1:-create}"
    shift 2>/dev/null
    local cd_file="$HOME/.spawnpoint/.cd_path"
    rm -f "$cd_file"
    case "$cmd" in
        create)     spawnpoint create "$@" ;;
        list|ls)    spawnpoint list --cd "$@" ;;
        *)          spawnpoint "$cmd" "$@" ;;
    esac
    if [ -f "$cd_file" ]; then
        local dir=$(cat "$cd_file")
        rm -f "$cd_file"
        [ -n "$dir" ] && cd "$dir"
    fi
}
`

const fishSnippet = `
# spawnpoint shell integration
function sp
    set cmd (test (count $argv) -gt 0; and echo $argv[1]; or echo create)
    set rest $argv[2..]
    set cd_file "$HOME/.spawnpoint/.cd_path"
    rm -f $cd_file
    switch $cmd
        case create
            spawnpoint create $rest
        case list ls
            spawnpoint list --cd $rest
        case '*'
            spawnpoint $cmd $rest
    end
    if test -f $cd_file
        set dir (cat $cd_file)
        rm -f $cd_file
        test -n "$dir"; and cd $dir
    end
end
`

func shellRCFiles() []string {
	h, _ := os.UserHomeDir()
	j := func(p ...string) string { return filepath.Join(append([]string{h}, p...)...) }
	shell := os.Getenv("SHELL")
	var candidates []string
	switch {
	case strings.Contains(shell, "fish"):
		candidates = []string{j(".config", "fish", "config.fish")}
	case strings.Contains(shell, "bash"):
		candidates = []string{j(".bashrc"), j(".bash_profile"), j(".zshrc")}
	default:
		candidates = []string{j(".zshrc"), j(".bashrc"), j(".bash_profile")}
	}
	var out []string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func offerShellIntegration() error {
	rcs := shellRCFiles()
	if len(rcs) == 0 {
		ui.Println(ui.Dim("Tip: add this to your shell rc to auto-cd after creating a workspace:"))
		ui.Println(strings.TrimSpace(posixSnippet))
		return nil
	}
	for _, rc := range rcs {
		if data, err := os.ReadFile(rc); err == nil && strings.Contains(string(data), shellMarker) {
			ui.Println(ui.Dim("Shell integration already present in " + tilde(rc)))
			return nil
		}
	}
	const skip = "Skip"
	choice, err := ui.Select("Add the sp() shell function for auto-cd after create?", append(tildeAll(rcs), skip), tilde(rcs[0]))
	if err != nil {
		return err
	}
	if choice == skip {
		ui.Println(ui.Dim("Skipped. You can add it manually:"))
		ui.Println(strings.TrimSpace(posixSnippet))
		return nil
	}
	rc := rcs[slices.Index(tildeAll(rcs), choice)]
	snippet := posixSnippet
	if strings.HasSuffix(rc, ".fish") {
		snippet = fishSnippet
	}
	f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fail("%v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(snippet); err != nil {
		return fail("%v", err)
	}
	ui.Success("Added sp() to %s", tilde(rc))
	ui.Hint("Restart your shell or run: source %s", tilde(rc))
	return nil
}

func configCmd() *cobra.Command {
	var edit, reset bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show, edit or reset the config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch {
			case reset:
				ok, err := ui.Confirm("Reset config to defaults?", false)
				if err != nil || !ok {
					return err
				}
				cfg := config.Default()
				cfg.ScanDirs = config.DetectScanDirs()
				if _, err := config.Save(cfg); err != nil {
					return fail("%v", err)
				}
				ui.Success("Config reset to defaults.")
				return nil
			case edit:
				if !config.Exists() {
					return fail("No config file yet. Run %s first.", ui.Bold("spawnpoint init"))
				}
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "vi"
					for _, e := range []string{"vim", "nano"} {
						if _, err := exec.LookPath(e); err == nil {
							editor = e
							break
						}
					}
				}
				parts := strings.Fields(editor)
				c := exec.Command(parts[0], append(parts[1:], config.Path())...)
				c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
				return c.Run()
			}
			data, err := os.ReadFile(config.Path())
			if err != nil {
				ui.Warning("No config file. Run %s to create one.", ui.Bold("spawnpoint init"))
				return nil
			}
			ui.Println(ui.Bold("Config") + " " + ui.Dim(tilde(config.Path())))
			ui.Blank()
			fmt.Print(string(data))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open the config in $EDITOR")
	cmd.Flags().BoolVar(&reset, "reset", false, "reset the config to defaults")
	return cmd
}
