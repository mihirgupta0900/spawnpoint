package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/workspace"
)

func addCmd() *cobra.Command {
	var (
		af          agentFlags
		repos, base string
	)
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add repos to the current workspace",
		Long:    "Add repos to the workspace you're in, on the same branch. A single-repo\nworkspace is converted to the multi-repo layout automatically.",
		Example: "cd ~/.spawnpoint/workspaces/feat-billing && sp add\nspawnpoint add --no-input --json --repos worker",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(af)
			if err != nil {
				return err
			}
			return runAdd(cfg, af, repos, base)
		},
	}
	af.register(cmd, "never prompt; requires --repos")
	cmd.Flags().StringVar(&repos, "repos", "", "comma-separated repo names to add")
	cmd.Flags().StringVar(&base, "base", "", "base branch for new branches (default: each repo's default branch)")
	return cmd
}

func runAdd(cfg *config.Config, af agentFlags, reposArg, base string) error {
	cwd, _ := os.Getwd()
	ws, branch, ok := workspace.Detect(cwd, cfg.WorktreeDirs())
	if !ok {
		ui.Error("Not inside a spawnpoint workspace.")
		ui.Hint("Run %s to create one, or cd into an existing workspace first.", ui.Bold("spawnpoint create"))
		return exit(1)
	}
	existing := workspace.RepoNamesIn(ws)
	ui.Println(ui.Accent("Workspace ") + ui.Bold(filepath.Base(ws)) + ui.Dim("  on ") + ui.Accent(branch))
	if len(existing) > 0 {
		ui.Hint("already here: %s", strings.Join(sortedKeys(existing), ", "))
	}

	all, _, err := scanRepos(cfg)
	if err != nil {
		return err
	}
	var candidates []workspace.Repo
	for _, r := range all {
		if !existing[r.Base()] {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		ui.Warning("No other repositories to add.")
		return nil
	}

	var selected []workspace.Repo
	if af.noInput {
		if err := requireFlag(reposArg, "--repos"); err != nil {
			return err
		}
		if selected, err = resolveRepos(parseCSV(reposArg), candidates, ""); err != nil {
			return err
		}
	} else {
		var pre []string
		if reposArg != "" {
			if selected, err = resolveRepos(parseCSV(reposArg), candidates, ""); err != nil {
				return err
			}
			pre = repoNames(selected)
		}
		labels, err := ui.Pick(ui.PickOptions{Title: "Select repositories to add", Items: repoNames(candidates), Selected: pre, Multi: true})
		if err != nil {
			return err
		}
		if len(labels) == 0 {
			ui.Println("No repositories selected.")
			return nil
		}
		selected = reposByName(candidates, labels)
	}

	restructure := workspace.IsSingleRepo(ws)
	prepare(selected)

	actions, err := planActions(selected, planOpts{
		branch: branch, base: base, noInput: af.noInput,
		target: func(r workspace.Repo) string { return filepath.Join(ws, r.Base()) },
	})
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		ui.Warning("Nothing to do.")
		return nil
	}

	if restructure {
		ui.Blank()
		ui.Println(ui.Warn("Note:") + " this single-repo workspace will switch to the multi-repo layout.")
		ui.Hint("The existing worktree moves into %s/", filepath.Join(filepath.Base(ws), workspace.SingleRepoName(ws)))
	}
	printPlan(actions)
	if !af.noInput {
		ok, err := ui.Confirm("Proceed?", true)
		if err != nil {
			return err
		}
		if !ok {
			ui.Println(ui.Dim("Aborted."))
			return nil
		}
	}

	if restructure {
		name, err := workspace.Restructure(ws)
		if err != nil {
			return fail("restructuring workspace: %v", err)
		}
		ui.Success("Moved the existing worktree into %s/", name)
	}

	build(actions, cfg, "added")
	ui.Done("Workspace: " + ui.Accent(tilde(ws)))

	if af.json {
		return emit(struct {
			Workspace string       `json:"workspace"`
			Branch    string       `json:"branch"`
			Added     []repoResult `json:"added"`
		}{ws, branch, results(actions)})
	}
	return nil
}
