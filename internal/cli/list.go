package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

func listCmd() *cobra.Command {
	var (
		af   agentFlags
		cd   bool
		name string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces (or pick one to cd into)",
		Example: "sp list\nsp list --cd\nspawnpoint list --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			return runList(cfg, af, cd, name)
		},
	}
	cmd.Flags().BoolVarP(&cd, "cd", "c", false, "pick a workspace to cd into (with shell integration)")
	af.register(cmd, "never prompt; with --cd, requires --workspace")
	cmd.Flags().StringVar(&name, "workspace", "", "workspace to cd into (with --cd --no-input)")
	return cmd
}

type workspaceJSON struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Repos    int      `json:"repos"`
	Branches []string `json:"branches"`
	Dirty    bool     `json:"dirty"`
}

func workspacePayload(ws *workspace.Workspace) workspaceJSON {
	return workspaceJSON{ws.Name, ws.Path, len(ws.Worktrees), ws.Branches(), ws.Dirty()}
}

func runList(cfg *config.Config, af agentFlags, cd bool, name string) error {
	all := workspace.ScanWorkspaces(cfg.WorktreeDirs())
	workspace.SortByAge(all, true)

	if len(all) == 0 {
		if af.json {
			return emit([]workspaceJSON{})
		}
		ui.Warning("No workspaces yet. Create one with %s.", ui.Bold("sp create"))
		return nil
	}

	if !cd {
		if af.json {
			out := make([]workspaceJSON, len(all))
			for i, ws := range all {
				out[i] = workspacePayload(ws)
			}
			return emit(out)
		}
		rows := make([][]string, len(all))
		for i, ws := range all {
			status := ui.Good("clean")
			if ws.Dirty() {
				status = ui.Warn("dirty")
			}
			rows[i] = []string{
				ui.Bold(ws.Name), strconv.Itoa(len(ws.Worktrees)), strings.Join(ws.Branches(), ", "),
				status, ui.Dim(workspace.FormatAge(ws.Oldest())),
			}
		}
		ui.Table([]string{"Workspace", "Repos", "Branch", "Status", "Modified"}, rows)
		return nil
	}

	var chosen *workspace.Workspace
	if af.noInput {
		if err := requireFlag(name, "--workspace"); err != nil {
			return err
		}
		picked, err := resolveWorkspaces([]string{name}, all)
		if err != nil {
			return err
		}
		chosen = picked[0]
	} else {
		labels := make([]string, len(all))
		byLabel := map[string]*workspace.Workspace{}
		for i, ws := range all {
			labels[i] = ws.Label()
			byLabel[labels[i]] = ws
		}
		picked, err := ui.Pick(ui.PickOptions{Title: "Select workspace", Items: labels})
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			return nil
		}
		chosen = byLabel[picked[0]]
	}

	writeCDPath(chosen.Path)
	switch {
	case af.json:
		return emit(workspacePayload(chosen))
	case af.noInput:
		fmt.Println(chosen.Path)
	}
	return nil
}

func reposCmd() *cobra.Command {
	var af agentFlags
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "List repos you can pick (names for --repos)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			if len(cfg.ValidScanDirs()) == 0 {
				return fail("No scan directories configured. Run %s.", ui.Bold("spawnpoint init"))
			}
			repos := workspace.FindRepos(cfg.ValidScanDirs(), cfg.ScanDepth)
			if af.json {
				if repos == nil {
					repos = []workspace.Repo{}
				}
				return emit(repos)
			}
			if len(repos) == 0 {
				ui.Warning("No git repositories found.")
				return nil
			}
			rows := make([][]string, len(repos))
			for i, r := range repos {
				rows[i] = []string{ui.Bold(r.Name), ui.Dim(tilde(r.Path))}
			}
			ui.Table([]string{"Name", "Path"}, rows)
			return nil
		},
	}
	af.register(cmd, "")
	return cmd
}
